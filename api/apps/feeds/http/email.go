// Package feedshttp serves the feeds app's Resend inbound-email webhook.
package feedshttp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/services"
)

// resendSignatureMaxAge is the Svix replay window for svix-timestamp, either
// direction.
const resendSignatureMaxAge = 5 * time.Minute

// Handler serves the Resend inbound-email webhook.
type Handler struct {
	feeds  *services.FeedService
	logger *slog.Logger
	// secret verifies the webhook signature. configured is false when the
	// secret or the Resend API key is unset.
	secret     string
	configured bool
}

// NewHandler returns a Handler verifying signatures with secret. configured
// must be false when the Resend API key is unset too.
func NewHandler(
	feeds *services.FeedService,
	logger *slog.Logger,
	secret string,
	configured bool,
) *Handler {
	return &Handler{
		feeds:      feeds,
		logger:     logger,
		secret:     secret,
		configured: configured && secret != "",
	}
}

// Routes mounts the webhook at one static path; the feed is identified by
// the token in the "to" address. Auth is the webhook signature
// (verifyResendSignature), not AppAccess.
func (h *Handler) Routes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc("POST /"+prefix+"/email/inbound", h.inbound)
}

// resendInboundPayload is the "email.received" webhook body (metadata only;
// the body is fetched by the service).
type resendInboundPayload struct {
	Type string `json:"type"`
	Data struct {
		EmailID   string   `json:"email_id"`
		MessageID string   `json:"message_id"`
		From      string   `json:"from"`
		To        []string `json:"to"`
		// ReceivedFor is the address the route matched, which differs from
		// To when the alias was cc'd/bcc'd.
		ReceivedFor []string `json:"received_for"`
		Subject     string   `json:"subject"`
	} `json:"data"`
}

const resendEventEmailReceived = "email.received"

func (h *Handler) inbound(w http.ResponseWriter, r *http.Request) {
	if !h.configured {
		h.logger.ErrorContext(r.Context(),
			"email inbound: not configured (EMAIL_INBOUND_SECRET/RESEND_API_KEY unset)")
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.WarnContext(r.Context(), "email inbound: body read failed",
			"error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !verifyResendSignature(h.secret, r.Header, body) {
		h.logger.WarnContext(
			r.Context(),
			"email inbound: signature verification failed",
			"hasSvixID",
			r.Header.Get("svix-id") != "",
			"hasSvixTimestamp",
			r.Header.Get("svix-timestamp") != "",
			"hasSvixSignature",
			r.Header.Get("svix-signature") != "",
		)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload resendInboundPayload
	if err = json.Unmarshal(body, &payload); err != nil {
		h.logger.WarnContext(r.Context(), "email inbound: malformed body",
			"error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Ignore other event types so Resend never sees a failed delivery.
	if payload.Type == resendEventEmailReceived {
		h.feeds.HandleInboundEmail(r.Context(), services.InboundEmail{
			EmailID:     payload.Data.EmailID,
			MessageID:   payload.Data.MessageID,
			Subject:     payload.Data.Subject,
			To:          payload.Data.To,
			ReceivedFor: payload.Data.ReceivedFor,
		})
	}
	w.WriteHeader(http.StatusOK)
}

// verifyResendSignature verifies Resend's Svix scheme: HMAC-SHA256 over
// "{svix-id}.{svix-timestamp}.{body}" with the base64 secret after "whsec_";
// svix-signature holds space-separated "v1,<sig>" values, any may match. The
// timestamp must be within resendSignatureMaxAge.
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
