package reading_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/apps/reading"
	"tools.xdoubleu.com/apps/reading/internal/mocks"
	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

const emailWebhookSecret = "whsec_dGVzdC1zZWNyZXQtdGVzdC1zZWNyZXQ="

// signEmailWebhookBody signs body the way Resend signs "email.received"
// webhooks (Svix scheme) — see verifyResendSignature.
func signEmailWebhookBody(t *testing.T, secret, id string, body []byte) http.Header {
	t.Helper()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	require.NoError(t, err)

	mac := hmac.New(sha256.New, raw)
	mac.Write([]byte(id + "." + timestamp + "." + string(body)))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("svix-id", id)
	h.Set("svix-timestamp", timestamp)
	h.Set("svix-signature", "v1,"+sig)
	return h
}

type resendReceivingStub struct {
	server *httptest.Server
	html   string
	status int
}

func newResendReceivingStub(t *testing.T) *resendReceivingStub {
	t.Helper()
	//nolint:exhaustruct // server assigned right below, once the stub exists
	stub := &resendReceivingStub{html: "<p>Hello subscriber</p>", status: http.StatusOK}
	stub.server = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-resend-key", r.Header.Get("Authorization"))
			w.WriteHeader(stub.status)
			_ = json.NewEncoder(w).Encode(map[string]string{"html": stub.html})
		},
	))
	t.Cleanup(stub.server.Close)
	reading.SetResendAPIBaseURL(stub.server.URL)
	t.Cleanup(func() { reading.SetResendAPIBaseURL("https://api.resend.com") })
	return stub
}

// emailWebhookApp builds an email-configured app (ResendAPIKey set, unlike
// newEmailConfiguredApp's callers elsewhere) plus a raw handler for POSTing
// webhook bodies directly, and a connect client for creating the feed first.
func emailWebhookApp(t *testing.T) (http.Handler, *reading.Reading) {
	t.Helper()
	app, _ := newEmailConfiguredApp(t, emailWebhookSecret, "mail.test.example")
	cfg := app.Config
	cfg.ResendAPIKey = "test-resend-key"
	app.Config = cfg
	return testhelper.BuildMux(app), app
}

// emailWebhookAppFailingIngest is like emailWebhookApp but its HTML→EPUB
// converter always fails, so IngestEmail's error branch (log + persist
// last_error, no book created) can be exercised directly.
func emailWebhookAppFailingIngest(t *testing.T) (http.Handler, *reading.Reading) {
	t.Helper()
	cfg := testCfg
	cfg.EmailInboundSecret = emailWebhookSecret
	cfg.EmailInboundDomain = "mail.test.example"
	cfg.ResendAPIKey = "test-resend-key"

	clients := reading.Clients{
		UniCat:      nil,
		Hardcover:   mocks.NewMockHardcoverClient(),
		ObjectStore: objectstore.NewFake(),
		WebFetch:    mocks.NewMockWebFetchClient(),
		Arxiv:       mocks.NewMockArxivClient(),
		HTMLConvert: func(
			_ context.Context, _, _ string, _ services.ArticleMeta,
		) error {
			return errors.New("boom: converter unavailable")
		},
		KoboStoreBaseURL: "",
		PublicAPIBaseURL: "",
	}
	app := reading.NewInner(
		sharedmocks.NewMockedAuthService(userID),
		logging.NewNopLogger(),
		cfg,
		testDB,
		clients,
	)
	return testhelper.BuildMux(app), app
}

func createEmailFeedFor(t *testing.T, mux http.Handler) (string, string) {
	t.Helper()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := newBooksClientFor(ts.URL, connect.WithHTTPGet())

	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(
			t,
			&readingv1.CreateFeedRequest{Kind: readingv1.FeedKind_FEED_KIND_EMAIL},
		),
	)
	require.NoError(t, err)

	addr := resp.Msg.Feed.InboundAddress
	local, _, ok := strings.Cut(addr, "@")
	require.True(t, ok, "inbound address must contain @: %s", addr)
	_, tok, ok := strings.Cut(local, "+")
	require.True(t, ok, "inbound address local part must contain +: %s", addr)
	return resp.Msg.Feed.Id, tok
}

func postWebhook(
	mux http.Handler,
	body []byte,
	headers http.Header,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		http.MethodPost,
		"/reading/email/inbound",
		bytes.NewReader(body),
	)
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// messageIDFor mirrors inboundPayload's message_id derivation — the ingest
// dedup key uses message_id (falling back to email_id only when absent), so
// tests asserting on the resulting SourceURL must use this, not the raw
// emailID.
func messageIDFor(emailID string) string {
	return "<" + emailID + "@example.com>"
}

func inboundPayload(to, subject, emailID string) []byte {
	payload, _ := json.Marshal(map[string]any{
		"type": "email.received",
		"data": map[string]any{
			"email_id":   emailID,
			"message_id": messageIDFor(emailID),
			"from":       "newsletter@example.com",
			"to":         []string{to},
			"subject":    subject,
		},
	})
	return payload
}

// TestEmailInbound_ValidSignature_IngestsBook proves the full happy path:
// a correctly signed email.received webhook for a known feed alias fetches
// the email body from Resend and lands it as a library book.
func TestEmailInbound_ValidSignature_IngestsBook(t *testing.T) {
	mux, app := emailWebhookApp(t)
	feedID, token := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Fresh newsletter content</p>"

	to := "reading+" + token + "@mail.test.example"
	body := inboundPayload(to, "This week's issue", "email-1")
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-1", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	book, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(), "mailto:"+feedID+"/"+messageIDFor("email-1"),
	)
	require.NoError(t, err)
	assert.Equal(t, "This week's issue", book.Title)
}

// TestEmailInbound_InvalidSignature_Rejected proves a bad signature is
// rejected before anything is ingested.
func TestEmailInbound_InvalidSignature_Rejected(t *testing.T) {
	mux, app := emailWebhookApp(t)
	_, token := createEmailFeedFor(t, mux)
	newResendReceivingStub(t)

	to := "reading+" + token + "@mail.test.example"
	body := inboundPayload(to, "Should not land", "email-2")
	headers := signEmailWebhookBody(t, "whsec_d3Jvbmctc2VjcmV0LWhlcmUh", "msg-2", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	_, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(), "mailto:x/email-2",
	)
	assert.Error(t, err)
}

// TestEmailInbound_UnknownToken_NoOp proves an alias that doesn't resolve to
// any feed is acknowledged (200, so Resend doesn't retry-storm) without
// ingesting anything.
func TestEmailInbound_UnknownToken_NoOp(t *testing.T) {
	mux, _ := emailWebhookApp(t)
	newResendReceivingStub(t)

	to := "reading+not-a-real-token@mail.test.example"
	body := inboundPayload(to, "Nobody home", "email-3")
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-3", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestEmailInbound_MalformedBody_BadRequest proves a body that isn't valid
// JSON is rejected cleanly rather than panicking.
func TestEmailInbound_MalformedBody_BadRequest(t *testing.T) {
	mux, _ := emailWebhookApp(t)
	body := []byte("not json")
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-4", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestEmailInbound_SecretUnset_Rejected proves the webhook never accepts
// requests when EMAIL_INBOUND_SECRET is unset — testApp's default config,
// used directly here instead of emailWebhookApp's configured variant.
func TestEmailInbound_SecretUnset_Rejected(t *testing.T) {
	body := inboundPayload("reading+whatever@mail.test.example", "x", "email-5")
	rec := postWebhook(
		getRoutes(),
		body,
		http.Header{"Content-Type": {"application/json"}},
	)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestEmailInbound_SourceURLUniqueness proves the same Resend message id
// ingested for two different feeds produces two distinct dedup keys (feed ID
// is part of SourceURL), rather than the second silently deduping onto the
// first feed's book.
func TestEmailInbound_SourceURLUniqueness(t *testing.T) {
	mux, app := emailWebhookApp(t)
	feedA, tokenA := createEmailFeedFor(t, mux)
	feedB, tokenB := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Same message id, different feeds</p>"

	for _, tok := range []string{tokenA, tokenB} {
		to := "reading+" + tok + "@mail.test.example"
		body := inboundPayload(to, "Shared subject", "shared-email-id")
		headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-"+tok, body)
		rec := postWebhook(mux, body, headers)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	msgID := messageIDFor("shared-email-id")
	bookA, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(), "mailto:"+feedA+"/"+msgID,
	)
	require.NoError(t, err)
	bookB, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(), "mailto:"+feedB+"/"+msgID,
	)
	require.NoError(t, err)
	assert.NotEqual(t, bookA.ID, bookB.ID)
}

// TestEmailInbound_IngestFailure_RecordsLastError proves that when ingesting
// the email body fails, the webhook still acks 200 (Resend must not retry a
// permanently-broken email forever) but the feed's last_error is recorded
// for the user to see. The catalog row itself is still created — as with RSS
// ingest, the book metadata and extracted HTML are persisted before the EPUB
// build runs, so only the EPUB build step fails.
func TestEmailInbound_IngestFailure_RecordsLastError(t *testing.T) {
	mux, app := emailWebhookAppFailingIngest(t)
	feedID, token := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>This will fail to convert</p>"

	to := "reading+" + token + "@mail.test.example"
	body := inboundPayload(to, "Broken issue", "email-fail")
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-fail", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	feed, err := app.Repositories.Feeds.GetByID(
		context.Background(),
		userID,
		mustParseUUID(t, feedID),
	)
	require.NoError(t, err)
	require.NotNil(t, feed.LastError)
	assert.Contains(t, *feed.LastError, "boom: converter unavailable")
}

// TestEmailInbound_KoboSyncFeed_EnablesSync proves that an email feed created
// with kobo_sync=true opts newly ingested items into Kobo sync, mirroring
// the same auto-enable behaviour polled RSS feeds already have.
func TestEmailInbound_KoboSyncFeed_EnablesSync(t *testing.T) {
	mux, app := emailWebhookApp(t)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := newBooksClientFor(ts.URL, connect.WithHTTPGet())

	resp, err := client.CreateFeed(
		context.Background(),
		feedReq(t, &readingv1.CreateFeedRequest{
			Kind:     readingv1.FeedKind_FEED_KIND_EMAIL,
			KoboSync: true,
		}),
	)
	require.NoError(t, err)
	local, _, _ := strings.Cut(resp.Msg.Feed.InboundAddress, "@")
	_, token, _ := strings.Cut(local, "+")

	stub := newResendReceivingStub(t)
	stub.html = "<p>Kobo-synced newsletter</p>"

	to := "reading+" + token + "@mail.test.example"
	body := inboundPayload(to, "Synced issue", "email-kobo")
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-kobo", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	book, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(),
		"mailto:"+resp.Msg.Feed.Id+"/"+messageIDFor("email-kobo"),
	)
	require.NoError(t, err)
	ub, err := app.Repositories.Books.GetUserBook(context.Background(), userID, book.ID)
	require.NoError(t, err)
	assert.Contains(t, ub.Tags, models.TagKoboSync)
}

// TestEmailInbound_ResendFetchFails_NoOp proves that when the follow-up
// "retrieve received email" call to Resend fails (e.g. Resend having an
// outage), the webhook still acks 200 rather than surfacing the failure to
// Resend as a delivery error, nothing is ingested, and the failure is
// recorded as the feed's last_error so the user isn't left staring at
// "Waiting for your first email…" with no diagnostic at all.
func TestEmailInbound_ResendFetchFails_NoOp(t *testing.T) {
	mux, app := emailWebhookApp(t)
	feedID, token := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.status = http.StatusInternalServerError

	to := "reading+" + token + "@mail.test.example"
	body := inboundPayload(to, "Never arrives", "email-fetchfail")
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-fetchfail", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	_, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(),
		"mailto:"+feedID+"/"+messageIDFor("email-fetchfail"),
	)
	assert.Error(t, err, "a failed Resend fetch must not ingest anything")

	feed, err := app.Repositories.Feeds.GetByID(
		context.Background(), userID, mustParseUUID(t, feedID),
	)
	require.NoError(t, err)
	require.NotNil(t, feed.LastError)
	assert.Contains(t, *feed.LastError, "500")
}

// TestEmailInbound_IgnoredEventType_NoOp proves that a webhook event other
// than "email.received" (the only one this endpoint is subscribed to) is
// acked 200 and ignored, rather than erroring — so the same endpoint can be
// reused for other Resend event types later without breaking delivery.
func TestEmailInbound_IgnoredEventType_NoOp(t *testing.T) {
	mux, _ := emailWebhookApp(t)
	_, token := createEmailFeedFor(t, mux)

	to := "reading+" + token + "@mail.test.example"
	payload, _ := json.Marshal(map[string]any{
		"type": "email.bounced",
		"data": map[string]any{
			"email_id": "email-ignored",
			"to":       []string{to},
		},
	})
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-ignored", payload)

	rec := postWebhook(mux, payload, headers)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestEmailInbound_MissingMessageID_FallsBackToEmailID proves the dedup key
// falls back to Resend's email_id when message_id is absent from the
// payload — some inbound sources may not always populate it.
func TestEmailInbound_MissingMessageID_FallsBackToEmailID(t *testing.T) {
	mux, app := emailWebhookApp(t)
	feedID, token := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>No message id in payload</p>"

	to := "reading+" + token + "@mail.test.example"
	payload, _ := json.Marshal(map[string]any{
		"type": "email.received",
		"data": map[string]any{
			"email_id": "email-no-msgid",
			"to":       []string{to},
			"subject":  "No message id",
		},
	})
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-no-msgid", payload)

	rec := postWebhook(mux, payload, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	book, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(), "mailto:"+feedID+"/email-no-msgid",
	)
	require.NoError(t, err)
	assert.Equal(t, "No message id", book.Title)
}

// TestEmailInbound_SkipsNonMatchingToAddress proves resolveEmailFeed keeps
// scanning the "to" list past an address that isn't a "reading+<token>@…"
// alias (or doesn't resolve to any feed) instead of bailing out on the first
// candidate.
func TestEmailInbound_SkipsNonMatchingToAddress(t *testing.T) {
	mux, app := emailWebhookApp(t)
	feedID, token := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Found on the second address</p>"

	to := "reading+" + token + "@mail.test.example"
	payload, _ := json.Marshal(map[string]any{
		"type": "email.received",
		"data": map[string]any{
			"email_id":   "email-multi-to",
			"message_id": messageIDFor("email-multi-to"),
			"to":         []string{"not-an-alias@example.com", to},
			"subject":    "Multiple recipients",
		},
	})
	headers := signEmailWebhookBody(t, emailWebhookSecret, "msg-multi-to", payload)

	rec := postWebhook(mux, payload, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	book, err := app.Repositories.Books.GetBookBySourceURL(
		context.Background(), "mailto:"+feedID+"/"+messageIDFor("email-multi-to"),
	)
	require.NoError(t, err)
	assert.Equal(t, "Multiple recipients", book.Title)
}

func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}
