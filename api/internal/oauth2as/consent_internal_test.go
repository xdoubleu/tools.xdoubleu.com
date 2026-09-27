package oauth2as

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConsentToken(t *testing.T) {
	secret := []byte("secret")
	q := url.Values{"client_id": {"c"}, "state": {"s"}}
	now := time.Now()
	token := newConsentToken(secret, "user", q, now)

	assert.True(t, verifyConsentToken(secret, "user", q, token, now))
	assert.False(t, verifyConsentToken(secret, "other-user", q, token, now))
	assert.False(t, verifyConsentToken([]byte("other"), "user", q, token, now))
	assert.False(t, verifyConsentToken(
		secret, "user", url.Values{"client_id": {"c2"}, "state": {"s"}}, token, now,
	))
	assert.False(t, verifyConsentToken(
		secret, "user", q, token, now.Add(consentTokenTTL+time.Minute),
	))
	assert.False(t, verifyConsentToken(secret, "user", q, "", now))
	assert.False(t, verifyConsentToken(secret, "user", q, "x.y", now))
}
