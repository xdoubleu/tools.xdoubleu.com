package oauth2as

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ConsentTokenHeader carries the consent token on the approving POST.
//
//nolint:gosec // a header name, not a credential
const ConsentTokenHeader = "X-OAuth-Consent-Token"

const consentTokenTTL = 10 * time.Minute

// consentBoundParams are the authorize parameters a consent token commits to.
//
//nolint:gochecknoglobals //read-only list
var consentBoundParams = []string{
	"response_type", "client_id", "redirect_uri", "scope", "state",
	"code_challenge", "code_challenge_method", "nonce",
}

// newConsentToken binds an approval to one user and one authorize request,
// so only the consent page, which fetched it with the user's session, can
// complete authorization.
func newConsentToken(secret []byte, userID string, q url.Values, now time.Time) string {
	exp := strconv.FormatInt(now.Add(consentTokenTTL).Unix(), 10)
	return exp + "." + consentMAC(secret, userID, q, exp)
}

func verifyConsentToken(
	secret []byte, userID string, q url.Values, token string, now time.Time,
) bool {
	exp, mac, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || now.Unix() > expUnix {
		return false
	}
	return hmac.Equal([]byte(mac), []byte(consentMAC(secret, userID, q, exp)))
}

func consentMAC(secret []byte, userID string, q url.Values, exp string) string {
	h := hmac.New(sha256.New, secret)
	// Length-prefixed so no field value can shift another's boundary.
	write := func(v string) {
		h.Write([]byte(strconv.Itoa(len(v)) + ":" + v))
	}
	write("oauth2-consent")
	write(userID)
	write(exp)
	for _, name := range consentBoundParams {
		write(q.Get(name))
	}
	return hex.EncodeToString(h.Sum(nil))
}
