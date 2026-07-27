package reading

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestInboundTokenFromAddress(t *testing.T) {
	tests := []struct {
		name      string
		addr      string
		wantToken string
		wantOK    bool
	}{
		{"valid alias", "reading+abc123@mail.example.com", "abc123", true},
		{"no plus", "reading@mail.example.com", "", false},
		{"no at", "reading+abc123", "", false},
		{"empty token", "reading+@mail.example.com", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := inboundTokenFromAddress(tt.addr)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}
