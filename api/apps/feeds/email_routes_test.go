package feeds_test

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
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/feeds"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
)

// emailWebhookSecret is built at runtime so secret scanners don't flag a
// whsec_-shaped literal; a function to satisfy gochecknoglobals.
func emailWebhookSecret() string {
	return fakeWebhookSecret("FAKE-SECRET-FOR-TESTS-DO-NOT-USE")
}

// fakeWebhookSecret builds a valid "whsec_"+base64 secret from a seed.
func fakeWebhookSecret(seed string) string {
	return "whsec_" + base64.StdEncoding.EncodeToString([]byte(seed))
}

// signEmailWebhookBody signs body with Resend's Svix scheme.
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
	feeds.SetResendAPIBaseURL(stub.server.URL)
	t.Cleanup(func() { feeds.SetResendAPIBaseURL("https://api.resend.com") })
	return stub
}

func createEmailFeedFor(
	t *testing.T, mux http.Handler,
) (string, string, feedsv1connect.FeedServiceClient) {
	t.Helper()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client := feedsv1connect.NewFeedServiceClient(http.DefaultClient, ts.URL)

	resp, err := client.CreateFeed(
		context.Background(),
		connect.NewRequest(&feedsv1.CreateFeedRequest{
			Kind: feedsv1.FeedKind_FEED_KIND_EMAIL,
		}),
	)
	require.NoError(t, err)

	addr := resp.Msg.Feed.InboundAddress
	local, _, ok := strings.Cut(addr, "@")
	require.True(t, ok, "inbound address must contain @: %s", addr)
	return resp.Msg.Feed.Id, local, client
}

func postWebhook(
	mux http.Handler,
	body []byte,
	headers http.Header,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(
		http.MethodPost,
		"/feeds/email/inbound",
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

// messageIDFor mirrors inboundPayload's message_id, the ingest dedup key.
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

// itemBySourceURL polls ListFeedItems for an item with the given source URL.
func itemBySourceURL(
	t *testing.T, client feedsv1connect.FeedServiceClient, sourceURL string,
) *feedsv1.Item {
	t.Helper()
	resp, err := client.ListFeedItems(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedItemsRequest{}),
	)
	require.NoError(t, err)
	for _, item := range resp.Msg.Items {
		if item.SourceUrl == sourceURL {
			return item
		}
	}
	return nil
}

// TestEmailInbound_ValidSignature_IngestsItem covers the full happy path.
func TestEmailInbound_ValidSignature_IngestsItem(t *testing.T) {
	mux := getRoutes()
	feedID, token, client := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Fresh newsletter content</p>"

	to := token + "@mail.example.com"
	body := inboundPayload(to, "This week's issue", "email-1")
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-1", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	sourceURL := "mailto:" + feedID + "/" + messageIDFor("email-1")
	item := itemBySourceURL(t, client, sourceURL)
	require.NotNil(t, item)
	assert.Equal(t, "This week's issue", item.Title)
	assert.True(t, item.HasContent)
	assert.Contains(
		t, fetchItemContent(t, client, item.Id), "Fresh newsletter content",
	)
}

// TestEmailInbound_BlankSubject_DefaultsTitle: a blank subject gets the
// default title.
func TestEmailInbound_BlankSubject_DefaultsTitle(t *testing.T) {
	mux := getRoutes()
	feedID, token, client := createEmailFeedFor(t, mux)
	newResendReceivingStub(t)

	to := token + "@mail.example.com"
	body := inboundPayload(to, "   ", "email-blank-subject")
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-blank", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	sourceURL := "mailto:" + feedID + "/" + messageIDFor("email-blank-subject")
	item := itemBySourceURL(t, client, sourceURL)
	require.NotNil(t, item)
	assert.Equal(t, "Email newsletter", item.Title)
}

func TestEmailInbound_InvalidSignature_Rejected(t *testing.T) {
	mux := getRoutes()
	_, token, client := createEmailFeedFor(t, mux)
	newResendReceivingStub(t)

	to := token + "@mail.example.com"
	body := inboundPayload(to, "Should not land", "email-2")
	headers := signEmailWebhookBody(
		t,
		fakeWebhookSecret("FAKE-WRONG-SECRET-FOR-TESTS"),
		"msg-2",
		body,
	)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Nil(t, itemBySourceURL(t, client, "mailto:x/email-2"))
}

func TestEmailInbound_UnknownToken_NoOp(t *testing.T) {
	mux := getRoutes()
	newResendReceivingStub(t)

	to := "not-a-real-token@mail.example.com"
	body := inboundPayload(to, "Nobody home", "email-3")
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-3", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEmailInbound_MalformedBody_BadRequest(t *testing.T) {
	mux := getRoutes()
	body := []byte("not json")
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-4", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEmailInbound_SourceURLUniqueness(t *testing.T) {
	mux := getRoutes()
	feedA, tokenA, client := createEmailFeedFor(t, mux)
	feedB, tokenB, _ := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Same message id, different feeds</p>"

	for _, tok := range []string{tokenA, tokenB} {
		to := tok + "@mail.example.com"
		body := inboundPayload(to, "Shared subject", "shared-email-id")
		headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-"+tok, body)
		rec := postWebhook(mux, body, headers)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	msgID := messageIDFor("shared-email-id")
	itemA := itemBySourceURL(t, client, "mailto:"+feedA+"/"+msgID)
	itemB := itemBySourceURL(t, client, "mailto:"+feedB+"/"+msgID)
	require.NotNil(t, itemA)
	require.NotNil(t, itemB)
	assert.NotEqual(t, itemA.Id, itemB.Id)
}

// TestEmailInbound_ResendFetchFails_NoOp: a failed body fetch still acks 200,
// ingests nothing, and records last_error.
func TestEmailInbound_ResendFetchFails_NoOp(t *testing.T) {
	mux := getRoutes()
	feedID, token, client := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.status = http.StatusInternalServerError

	to := token + "@mail.example.com"
	body := inboundPayload(to, "Never arrives", "email-fetchfail")
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-fetchfail", body)

	rec := postWebhook(mux, body, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	assert.Nil(t, itemBySourceURL(
		t, client, "mailto:"+feedID+"/"+messageIDFor("email-fetchfail"),
	))

	list, err := client.ListFeeds(
		context.Background(), connect.NewRequest(&feedsv1.ListFeedsRequest{}),
	)
	require.NoError(t, err)
	var found *feedsv1.Feed
	for _, f := range list.Msg.Feeds {
		if f.Id == feedID {
			found = f
		}
	}
	require.NotNil(t, found)
	assert.Contains(t, found.LastError, "500")
}

// TestEmailInbound_Resend_RestoresDismissedItem: resending the same email
// un-dismisses the item via ON CONFLICT.
func TestEmailInbound_Resend_RestoresDismissedItem(t *testing.T) {
	mux := getRoutes()
	feedID, token, client := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Computer club newsletter</p>"

	to := token + "@mail.example.com"
	body := inboundPayload(to, "Issue #1", "email-resend")
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-resend-1", body)
	rec := postWebhook(mux, body, headers)
	require.Equal(t, http.StatusOK, rec.Code)

	sourceURL := "mailto:" + feedID + "/" + messageIDFor("email-resend")
	item := itemBySourceURL(t, client, sourceURL)
	require.NotNil(t, item)

	_, err := client.UpdateItem(
		context.Background(),
		connect.NewRequest(&feedsv1.UpdateItemRequest{
			ItemId:    item.Id,
			Dismissed: proto.Bool(true),
		}),
	)
	require.NoError(t, err)

	rec = postWebhook(mux, body, headers)
	require.Equal(t, http.StatusOK, rec.Code)

	restored := itemBySourceURL(t, client, sourceURL)
	require.NotNil(t, restored)
	assert.False(t, restored.Dismissed)
	assert.Empty(t, restored.ReadAt)
}

func TestRecordEmailFetchFailure_DBErrors_LogsWithoutPanicking(t *testing.T) {
	mux := getRoutes()
	feedID, _, _ := createEmailFeedFor(t, mux)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	parsed, err := uuid.Parse(feedID)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		testApp.Services.Feeds.RecordEmailFetchFailure(ctx, parsed, errors.New("boom"))
	})
}

func TestEmailInbound_IgnoredEventType_NoOp(t *testing.T) {
	mux := getRoutes()
	_, token, _ := createEmailFeedFor(t, mux)

	to := token + "@mail.example.com"
	payload, _ := json.Marshal(map[string]any{
		"type": "email.bounced",
		"data": map[string]any{
			"email_id": "email-ignored",
			"to":       []string{to},
		},
	})
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-ignored", payload)

	rec := postWebhook(mux, payload, headers)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestEmailInbound_MissingMessageID_FallsBackToEmailID(t *testing.T) {
	mux := getRoutes()
	feedID, token, client := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>No message id in payload</p>"

	to := token + "@mail.example.com"
	payload, _ := json.Marshal(map[string]any{
		"type": "email.received",
		"data": map[string]any{
			"email_id": "email-no-msgid",
			"to":       []string{to},
			"subject":  "No message id",
		},
	})
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-no-msgid", payload)

	rec := postWebhook(mux, payload, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	item := itemBySourceURL(t, client, "mailto:"+feedID+"/email-no-msgid")
	require.NotNil(t, item)
	assert.Equal(t, "No message id", item.Title)
}

// TestEmailInbound_ReceivedForFallback_IngestsItem: "received_for" is tried.
func TestEmailInbound_ReceivedForFallback_IngestsItem(t *testing.T) {
	mux := getRoutes()
	feedID, token, client := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Found via received_for</p>"

	to := token + "@mail.example.com"
	payload, _ := json.Marshal(map[string]any{
		"type": "email.received",
		"data": map[string]any{
			"email_id":     "email-received-for",
			"message_id":   messageIDFor("email-received-for"),
			"to":           []string{"someone-else@example.com"},
			"received_for": []string{to},
			"subject":      "Via received_for",
		},
	})
	headers := signEmailWebhookBody(
		t,
		emailWebhookSecret(),
		"msg-received-for",
		payload,
	)

	rec := postWebhook(mux, payload, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	item := itemBySourceURL(
		t, client, "mailto:"+feedID+"/"+messageIDFor("email-received-for"),
	)
	require.NotNil(t, item)
	assert.Equal(t, "Via received_for", item.Title)
}

// TestEmailInbound_SkipsNonMatchingToAddress: scanning continues past a
// non-matching "to" address.
func TestEmailInbound_SkipsNonMatchingToAddress(t *testing.T) {
	mux := getRoutes()
	feedID, token, client := createEmailFeedFor(t, mux)
	stub := newResendReceivingStub(t)
	stub.html = "<p>Found on the second address</p>"

	to := token + "@mail.example.com"
	payload, _ := json.Marshal(map[string]any{
		"type": "email.received",
		"data": map[string]any{
			"email_id":   "email-multi-to",
			"message_id": messageIDFor("email-multi-to"),
			"to":         []string{"not-an-alias@example.com", to},
			"subject":    "Multiple recipients",
		},
	})
	headers := signEmailWebhookBody(t, emailWebhookSecret(), "msg-multi-to", payload)

	rec := postWebhook(mux, payload, headers)
	assert.Equal(t, http.StatusOK, rec.Code)

	item := itemBySourceURL(
		t,
		client,
		"mailto:"+feedID+"/"+messageIDFor("email-multi-to"),
	)
	require.NotNil(t, item)
	assert.Equal(t, "Multiple recipients", item.Title)
}
