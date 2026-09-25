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

// recordCapture keeps every record, at every level, for assertions.
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

func attrs(record slog.Record) map[string]string {
	out := map[string]string{}
	record.Attrs(func(attr slog.Attr) bool {
		out[attr.Key] = attr.Value.String()
		return true
	})
	return out
}

// Refresh-token reuse past the grace period revokes the token family, so it
// must log at Error despite the 400.
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

	// Rotate, then age the old token past the grace period so replay is theft.
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

// A never-existing refresh token is routine and must stay at Warn.
func TestObserve_RefreshGrantNotFoundLogsAtWarn(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	srv.logs.reset()

	resp, _ := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"never-issued-refresh-token"},
		"client_id":     {client.ID},
	})
	require.NotEqual(t, http.StatusOK, resp.StatusCode)

	records := srv.logs.all()
	require.Len(t, records, 1)
	assert.Equal(t, slog.LevelWarn, records[0].Level)

	gotAttrs := attrs(records[0])
	assert.Equal(t, "refresh_token", gotAttrs["grant_type"])
	assert.Equal(t, "invalid_grant", gotAttrs["oauth_error"])
}

// Routine 4xx rejections must stay at Warn so they don't bury real signals.
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

func TestObserve_ConsentDeniedLogsAtWarn(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	_, challenge := pkcePair(t)
	srv.logs.reset()

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"observe-deny-1234"},
		"scope":                 {"offline_access"},
		"consent":               {"deny"},
	}
	resp, err := noRedirectClient().Get(s(srv) + "/oauth2/authorize?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()

	records := srv.logs.all()
	require.Len(t, records, 1)
	assert.Equal(t, slog.LevelWarn, records[0].Level)

	gotAttrs := attrs(records[0])
	assert.Equal(t, "/oauth2/authorize", gotAttrs["endpoint"])
	assert.Equal(t, "access_denied", gotAttrs["oauth_error"])
	assert.Empty(t, gotAttrs["grant_type"], "the authorize leg has no grant_type")
}

func TestObserve_MalformedRegistrationLogsAtWarn(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	srv.logs.reset()

	resp, err := http.Post(
		s(srv)+"/oauth2/register", "application/json",
		strings.NewReader("not json at all"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	records := srv.logs.all()
	require.Len(t, records, 1)
	assert.Equal(t, slog.LevelWarn, records[0].Level)
	assert.Equal(t, "/oauth2/register", attrs(records[0])["endpoint"])
}

func s(srv *oauth2asTestServer) string { return srv.ts.URL }

// The token form carries codes, PKCE verifiers and refresh tokens; none may
// reach a log line. Asserts on all attributes, not specific keys.
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

	const wrongClientSecret = "leaked-client-secret-should-not-appear"
	gClient := grafanaConfidentialClient(t, srv)
	gVerifier, gChallenge := pkcePair(t)
	gCode := srv.authorizeAndGetCodeWithScope(
		t, gClient, gChallenge, "observe-oidc-state1", "openid email",
	)
	srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {gCode},
		"redirect_uri":  {gClient.RedirectURIs[0]},
		"client_id":     {gClient.ID},
		"client_secret": {wrongClientSecret},
		"code_verifier": {gVerifier},
	})

	secrets := map[string]string{
		"access token":  out.AccessToken,
		"refresh token": out.RefreshToken,
		"auth code":     code,
		"PKCE verifier": verifier,
		"client secret": wrongClientSecret,
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
