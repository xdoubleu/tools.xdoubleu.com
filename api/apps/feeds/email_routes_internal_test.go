package feeds

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWebhookSecret = "whsec_dGVzdC1zZWNyZXQtdGVzdC1zZWNyZXQ="

// signedResendHeaders always signs with testWebhookSecret and svix-id
// "msg_1" — tests that need a mismatch verify against a different secret
// instead of signing with one.
func signedResendHeaders(body string, ts time.Time) http.Header {
	const id = "msg_1"
	timestamp := strconv.FormatInt(ts.Unix(), 10)
	raw, err := base64.StdEncoding.DecodeString(
		testWebhookSecret[len("whsec_"):],
	)
	if err != nil {
		panic(err)
	}
	mac := hmac.New(sha256.New, raw)
	mac.Write([]byte(id + "." + timestamp + "." + body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	h := http.Header{}
	h.Set("svix-id", id)
	h.Set("svix-timestamp", timestamp)
	h.Set("svix-signature", "v1,"+sig)
	return h
}

func TestVerifyResendSignature_Valid(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	headers := signedResendHeaders(string(body), time.Now())

	assert.True(t, verifyResendSignature(testWebhookSecret, headers, body))
}

func TestVerifyResendSignature_WrongSecret(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	headers := signedResendHeaders(string(body), time.Now())

	other := "whsec_ZGlmZmVyZW50LXNlY3JldC1oZXJlIQ=="
	assert.False(t, verifyResendSignature(other, headers, body))
}

func TestVerifyResendSignature_TamperedBody(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	headers := signedResendHeaders(string(body), time.Now())

	tampered := []byte(`{"type":"email.received","x":1}`)
	assert.False(t, verifyResendSignature(testWebhookSecret, headers, tampered))
}

func TestVerifyResendSignature_ExpiredTimestamp(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	old := time.Now().Add(-10 * time.Minute)
	headers := signedResendHeaders(string(body), old)

	assert.False(t, verifyResendSignature(testWebhookSecret, headers, body))
}

func TestVerifyResendSignature_MissingHeaders(t *testing.T) {
	assert.False(
		t,
		verifyResendSignature(testWebhookSecret, http.Header{}, []byte("{}")),
	)
}

func TestVerifyResendSignature_MalformedSecret(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	headers := signedResendHeaders(string(body), time.Now())

	assert.False(
		t,
		verifyResendSignature("whsec_not-valid-base64!!!", headers, body),
	)
}

func TestVerifyResendSignature_MalformedTimestamp(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	h := http.Header{}
	h.Set("svix-id", "msg_1")
	h.Set("svix-timestamp", "not-a-number")
	h.Set("svix-signature", "v1,doesnotmatter")

	assert.False(t, verifyResendSignature(testWebhookSecret, h, body))
}

func TestVerifyResendSignature_MalformedSignatureField(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	h := http.Header{}
	h.Set("svix-id", "msg_1")
	h.Set("svix-timestamp", timestamp)
	// No comma separating the version prefix from the signature — the
	// per-part parse loop must skip it (and any following valid part)
	// rather than panicking on strings.Cut's second return.
	h.Set("svix-signature", "no-comma-here")

	assert.False(t, verifyResendSignature(testWebhookSecret, h, body))
}

func TestParseUnixTimestamp_Invalid(t *testing.T) {
	_, err := parseUnixTimestamp("not-a-timestamp")
	require.Error(t, err)
}

func TestFetchReceivedEmailHTML_ConnectionError(t *testing.T) {
	SetResendAPIBaseURL("http://127.0.0.1:1")
	t.Cleanup(func() { SetResendAPIBaseURL("https://api.resend.com") })

	_, err := fetchReceivedEmailHTML(context.Background(), "key", "email-1")
	require.Error(t, err)
}

func TestFetchReceivedEmailHTML_NonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("upstream error"))
		},
	))
	t.Cleanup(server.Close)
	SetResendAPIBaseURL(server.URL)
	t.Cleanup(func() { SetResendAPIBaseURL("https://api.resend.com") })

	_, err := fetchReceivedEmailHTML(context.Background(), "key", "email-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream error")
}

func TestFetchReceivedEmailHTML_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("not json"))
		},
	))
	t.Cleanup(server.Close)
	SetResendAPIBaseURL(server.URL)
	t.Cleanup(func() { SetResendAPIBaseURL("https://api.resend.com") })

	_, err := fetchReceivedEmailHTML(context.Background(), "key", "email-1")
	require.Error(t, err)
}

func TestFetchReceivedEmailHTML_TextFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"text": "plain text body",
			})
		},
	))
	t.Cleanup(server.Close)
	SetResendAPIBaseURL(server.URL)
	t.Cleanup(func() { SetResendAPIBaseURL("https://api.resend.com") })

	html, err := fetchReceivedEmailHTML(context.Background(), "key", "email-1")
	require.NoError(t, err)
	assert.Equal(t, "<pre>plain text body</pre>", html)
}

func TestFetchReceivedEmailHTML_NoHTMLOrText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{})
		},
	))
	t.Cleanup(server.Close)
	SetResendAPIBaseURL(server.URL)
	t.Cleanup(func() { SetResendAPIBaseURL("https://api.resend.com") })

	_, err := fetchReceivedEmailHTML(context.Background(), "key", "email-1")
	require.Error(t, err)
}

func TestInboundTokenFromAddress(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		wantToken string
		wantOK    bool
	}{
		{"valid alias", "reading+abc123@mail.example.com", "abc123", true},
		// No "+" required: whole local part is the token.
		{"no plus", "abc123@mail.example.com", "abc123", true},
		{"no at", "reading+abc123", "", false},
		{"empty token after plus", "reading+@mail.example.com", "", false},
		{"empty local part", "@mail.example.com", "", false},
		// A relay lowercasing the recipient local-part shouldn't break
		// lookup — the token is folded to lowercase.
		{
			"mixed case folded to lowercase",
			"reading+AbC123@mail.example.com",
			"abc123",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := inboundTokenFromAddress(tt.addr)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
