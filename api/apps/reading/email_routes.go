package reading

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/models"
)

// emailUpstreamTimeout bounds the follow-up call to Resend's "retrieve
// received email" API (the inbound webhook itself only carries metadata —
// see fetchReceivedEmailHTML).
const emailUpstreamTimeout = 10 * time.Second

//nolint:gochecknoglobals // shared client, mirrors koboUpstreamClient
var emailUpstreamClient = &http.Client{Timeout: emailUpstreamTimeout}

//nolint:gochecknoglobals // overridable in tests, mirrors mailer.baseURL
var resendAPIBaseURL = "https://api.resend.com"

// SetResendAPIBaseURL overrides the Resend API base URL used to fetch a
// received email's body. Intended for tests only.
func SetResendAPIBaseURL(u string) { resendAPIBaseURL = u }

// resendSignatureMaxAge rejects a webhook whose svix-timestamp is further
// than this from now, in either direction — the standard Svix replay-attack
// guard (Resend's own webhooks.verify() enforces the same 5-minute window).
const resendSignatureMaxAge = 5 * time.Minute

// emailRoutes mounts the Resend inbound-email webhook (issue #595) at a
// single static path — Resend routes all inbound mail for the receiving
// domain to one webhook URL, so the *feed* is identified by the token
// embedded in the payload's "to" address, not the URL. AppAccess is NOT
// used: auth is Resend's own webhook signature instead (see
// verifyResendSignature).
func (app *Reading) emailRoutes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc("POST /"+prefix+"/email/inbound", app.emailInboundHandler)
}

// resendInboundPayload is the "email.received" webhook body. Resend sends
// metadata only here — the actual html/text body is fetched separately via
// fetchReceivedEmailHTML.
type resendInboundPayload struct {
	Type string `json:"type"`
	Data struct {
		EmailID   string   `json:"email_id"`
		MessageID string   `json:"message_id"`
		From      string   `json:"from"`
		To        []string `json:"to"`
		Subject   string   `json:"subject"`
	} `json:"data"`
}

const resendEventEmailReceived = "email.received"

func (app *Reading) emailInboundHandler(w http.ResponseWriter, r *http.Request) {
	if app.Config.EmailInboundSecret == "" || app.Config.ResendAPIKey == "" {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !verifyResendSignature(app.Config.EmailInboundSecret, r.Header, body) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload resendInboundPayload
	if err = json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Only email.received is subscribed to today, but a webhook endpoint can
	// be reused for other event types later — ignore anything else rather
	// than erroring, so Resend never sees this as a failed delivery.
	if payload.Type != resendEventEmailReceived {
		w.WriteHeader(http.StatusOK)
		return
	}

	feed := app.resolveEmailFeed(r.Context(), payload.Data.To)
	if feed == nil {
		app.Logger.WarnContext(r.Context(), "email inbound: unknown alias",
			"to", payload.Data.To)
		w.WriteHeader(http.StatusOK)
		return
	}

	htmlBody, err := fetchReceivedEmailHTML(
		r.Context(), app.Config.ResendAPIKey, payload.Data.EmailID,
	)
	if err != nil {
		app.Logger.ErrorContext(r.Context(), "email inbound: fetch body failed",
			"feedID", feed.ID, "emailID", payload.Data.EmailID, "error", err)
		app.Services.Feeds.RecordEmailFetchFailure(r.Context(), feed.ID, err)
		w.WriteHeader(http.StatusOK)
		return
	}

	messageID := payload.Data.MessageID
	if messageID == "" {
		messageID = payload.Data.EmailID
	}
	app.Services.Feeds.IngestEmail(
		r.Context(), *feed, messageID, payload.Data.Subject, htmlBody,
	)
	w.WriteHeader(http.StatusOK)
}

// resolveEmailFeed extracts the "reading+<token>@…" alias token from each
// candidate "to" address and returns the first one that resolves to a known
// email feed, or nil if none does.
func (app *Reading) resolveEmailFeed(
	ctx context.Context,
	to []string,
) *models.Feed {
	for _, addr := range to {
		token, ok := inboundTokenFromAddress(addr)
		if !ok {
			continue
		}
		h := sha256.Sum256([]byte(token))
		hash := hex.EncodeToString(h[:])
		feed, err := app.Services.Feeds.GetByInboundTokenHash(ctx, hash)
		if err == nil {
			return feed
		}
		if !errors.Is(err, database.ErrResourceNotFound) {
			app.Logger.WarnContext(ctx, "email inbound: token lookup failed",
				"error", err)
		}
	}
	return nil
}

// inboundTokenFromAddress extracts the token from a "reading+<token>@domain"
// address's local part.
func inboundTokenFromAddress(addr string) (string, bool) {
	local, _, ok := strings.Cut(addr, "@")
	if !ok {
		return "", false
	}
	_, token, ok := strings.Cut(local, "+")
	if !ok || token == "" {
		return "", false
	}
	return token, true
}

// verifyResendSignature verifies a Resend inbound webhook using the Svix
// scheme Resend delegates to: the signed content is
// "{svix-id}.{svix-timestamp}.{body}", HMAC-SHA256'd with the secret (after
// stripping its "whsec_" prefix and base64-decoding the remainder); the
// svix-signature header holds one or more space-separated "v1,<base64sig>"
// values, any of which may match. The timestamp is also required to be
// within resendSignatureMaxAge of now, guarding against replay.
func verifyResendSignature(secret string, headers http.Header, body []byte) bool {
	id := headers.Get("svix-id")
	timestamp := headers.Get("svix-timestamp")
	sigHeader := headers.Get("svix-signature")
	if id == "" || timestamp == "" || sigHeader == "" {
		return false
	}

	ts, err := parseUnixTimestamp(timestamp)
	if err != nil || time.Since(ts).Abs() > resendSignatureMaxAge {
		return false
	}

	rawSecret, err := base64.StdEncoding.DecodeString(
		strings.TrimPrefix(secret, "whsec_"),
	)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, rawSecret)
	mac.Write([]byte(id + "." + timestamp + "." + string(body)))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	for _, part := range strings.Fields(sigHeader) {
		_, sig, ok := strings.Cut(part, ",")
		if !ok {
			continue
		}
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return true
		}
	}
	return false
}

func parseUnixTimestamp(s string) (time.Time, error) {
	var sec int64
	if _, err := fmt.Sscanf(s, "%d", &sec); err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}

// resendReceivedEmail is the subset of the "retrieve received email"
// response (GET /emails/receiving/{id}) this app needs.
type resendReceivedEmail struct {
	HTML string `json:"html"`
	Text string `json:"text"`
}

// fetchReceivedEmailHTML retrieves the full body of a received email, since
// the "email.received" webhook payload carries metadata only. Falls back to
// the plain-text body (wrapped so it renders as paragraphs) when the email
// has no HTML part.
func fetchReceivedEmailHTML(
	ctx context.Context,
	apiKey, emailID string,
) (string, error) {
	url := resendAPIBaseURL + "/emails/receiving/" + emailID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := emailUpstreamClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf(
			"resend receiving API returned %d: %s", resp.StatusCode, string(raw),
		)
	}

	var received resendReceivedEmail
	if err = json.NewDecoder(resp.Body).Decode(&received); err != nil {
		return "", err
	}
	if received.HTML != "" {
		return received.HTML, nil
	}
	if received.Text != "" {
		return "<pre>" + received.Text + "</pre>", nil
	}
	return "", errors.New("received email has no html or text body")
}
