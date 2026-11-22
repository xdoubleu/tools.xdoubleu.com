package feedshttp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testWebhookSecret is built at runtime so secret scanners don't flag a
// whsec_-shaped literal; a function to satisfy gochecknoglobals.
func testWebhookSecret() string {
	return fakeWebhookSecret("FAKE-SECRET-FOR-TESTS-DO-NOT-USE")
}

// fakeWebhookSecret builds a valid "whsec_"+base64 secret from a seed.
func fakeWebhookSecret(seed string) string {
	return "whsec_" + base64.StdEncoding.EncodeToString([]byte(seed))
}

// signedResendHeaders signs with testWebhookSecret and svix-id "msg_1".
func signedResendHeaders(body string, ts time.Time) http.Header {
	const id = "msg_1"
	timestamp := strconv.FormatInt(ts.Unix(), 10)
	raw, err := base64.StdEncoding.DecodeString(
		testWebhookSecret()[len("whsec_"):],
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

	assert.True(t, verifyResendSignature(testWebhookSecret(), headers, body))
}

func TestVerifyResendSignature_WrongSecret(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	headers := signedResendHeaders(string(body), time.Now())

	other := fakeWebhookSecret("FAKE-DIFFERENT-SECRET-FOR-TESTS")
	assert.False(t, verifyResendSignature(other, headers, body))
}

func TestVerifyResendSignature_TamperedBody(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	headers := signedResendHeaders(string(body), time.Now())

	tampered := []byte(`{"type":"email.received","x":1}`)
	assert.False(t, verifyResendSignature(testWebhookSecret(), headers, tampered))
}

func TestVerifyResendSignature_ExpiredTimestamp(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	old := time.Now().Add(-10 * time.Minute)
	headers := signedResendHeaders(string(body), old)

	assert.False(t, verifyResendSignature(testWebhookSecret(), headers, body))
}

func TestVerifyResendSignature_MissingHeaders(t *testing.T) {
	assert.False(
		t,
		verifyResendSignature(testWebhookSecret(), http.Header{}, []byte("{}")),
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

	assert.False(t, verifyResendSignature(testWebhookSecret(), h, body))
}

func TestVerifyResendSignature_MalformedSignatureField(t *testing.T) {
	body := []byte(`{"type":"email.received"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	h := http.Header{}
	h.Set("svix-id", "msg_1")
	h.Set("svix-timestamp", timestamp)
	// A part without a comma must be skipped, not panic.
	h.Set("svix-signature", "no-comma-here")

	assert.False(t, verifyResendSignature(testWebhookSecret(), h, body))
}

func TestParseUnixTimestamp_Invalid(t *testing.T) {
	_, err := parseUnixTimestamp("not-a-timestamp")
	require.Error(t, err)
}
