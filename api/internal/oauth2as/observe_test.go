package oauth2as_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordCapture is a slog.Handler that keeps every record in memory so tests
// can assert on level and attributes. Enabled at every level so a severity
// assertion can't accidentally pass because the record was filtered out.
type recordCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func newRecordCapture() *recordCapture {
	return &recordCapture{mu: sync.Mutex{}, records: nil}
}

func (c *recordCapture) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (c *recordCapture) Handle(_ context.Context, record slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, record.Clone())
	return nil
}

func (c *recordCapture) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *recordCapture) WithGroup(_ string) slog.Handler      { return c }

func (c *recordCapture) all() []slog.Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]slog.Record(nil), c.records...)
}

func (c *recordCapture) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = nil
}

// attrs flattens one record's attributes into a map of key to string value.
func attrs(record slog.Record) map[string]string {
	out := map[string]string{}
	record.Attrs(func(attr slog.Attr) bool {
		out[attr.Key] = attr.Value.String()
		return true
	})
	return out
}

// TestObserve_RefreshGrantFailureLogsAtError is the alerting policy's whole
// point: a rejected refresh_token grant means a client that held a working
// session just lost it (issue #1177), so it must be Error — the level the
// root sentrytools module's LogHandler forwards to Sentry — even though the
// HTTP response is a 400.
func TestObserve_RefreshGrantFailureLogsAtError(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	verifier, challenge := pkcePair(t)
	code := srv.authorizeAndGetCode(t, client, challenge, "observe-state-1234")

	resp, out := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {verifier},
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, out.RefreshToken)

	// Rotate the refresh token, then age the rotated-out one past the reuse
	// grace period so replaying it is treated as theft rather than a retry —
	// the same setup handlers_test.go uses, and the closest stand-in for a
	// real client whose refresh has stopped working.
	_, rotated := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
	})
	require.NotEmpty(t, rotated.RefreshToken)

	tokenParts := strings.SplitN(out.RefreshToken, ".", 2)
	require.Len(t, tokenParts, 2)
	_, err := srv.db.Exec(context.Background(), `
		UPDATE auth.oauth2_refresh_tokens
		SET rotated_at = now() - interval '1 minute'
		WHERE signature = $1
	`, tokenParts[1])
	require.NoError(t, err)

	srv.logs.reset()

	staleResp, _ := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
	})
	require.NotEqual(t, http.StatusOK, staleResp.StatusCode)

	records := srv.logs.all()
	require.Len(t, records, 1, "the rejection must produce exactly one record")

	got := records[0]
	assert.Equal(t, slog.LevelError, got.Level)

	gotAttrs := attrs(got)
	assert.Equal(t, "/oauth2/token", gotAttrs["endpoint"])
	assert.Equal(t, "refresh_token", gotAttrs["grant_type"])
	assert.Equal(t, client.ID, gotAttrs["client_id"])
	assert.NotEmpty(t, gotAttrs["oauth_error"])
}

// TestObserve_AuthorizationCodeFailureLogsAtWarn guards the other half of the
// policy: routine 4xx rejections (a mistyped PKCE verifier here, but equally
// the steady background of scanners probing /oauth2/*) must stay at Warn, or
// they'd bury the refresh-grant signal above in Sentry noise.
func TestObserve_AuthorizationCodeFailureLogsAtWarn(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	_, challenge := pkcePair(t)
	code := srv.authorizeAndGetCode(t, client, challenge, "observe-state-5678")

	srv.logs.reset()

	resp, _ := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {"totally-the-wrong-verifier-0123456789"},
	})
	require.NotEqual(t, http.StatusOK, resp.StatusCode)

	records := srv.logs.all()
	require.Len(t, records, 1)
	assert.Equal(t, slog.LevelWarn, records[0].Level)
	assert.Equal(t, "authorization_code", attrs(records[0])["grant_type"])
}

// TestObserve_NeverLogsCredentials is the regression guard that matters most:
// the token endpoint's request form carries the authorization code, the PKCE
// verifier and the refresh token, and none of them may ever reach a log line.
// It asserts on the whole attribute set rather than on specific keys, so it
// still fires if fosite starts putting a credential in a field this code
// forwards verbatim (a hint, say).
func TestObserve_NeverLogsCredentials(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	verifier, challenge := pkcePair(t)
	code := srv.authorizeAndGetCode(t, client, challenge, "observe-state-9012")

	_, out := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {verifier},
	})
	require.NotEmpty(t, out.AccessToken)
	require.NotEmpty(t, out.RefreshToken)

	srv.logs.reset()

	// Drive several distinct rejections so the assertion covers more than one
	// error shape.
	srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken + "-tampered"},
		"client_id":     {client.ID},
	})
	srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {verifier},
	})

	secrets := map[string]string{
		"access token":  out.AccessToken,
		"refresh token": out.RefreshToken,
		"auth code":     code,
		"PKCE verifier": verifier,
	}

	records := srv.logs.all()
	require.NotEmpty(t, records, "expected the rejections to be logged")
	for _, record := range records {
		haystack := record.Message
		for key, value := range attrs(record) {
			haystack += " " + key + "=" + value
		}
		for name, secret := range secrets {
			assert.NotContains(t, haystack, secret, "log record leaked the %s", name)
		}
	}
}
