package oauth2as_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/fosite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/testhelper"
)

// noRedirectClient is an http.Client that never follows redirects, so tests
// can inspect a 302's Location header directly.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// pkcePair returns a random PKCE code_verifier and its S256 code_challenge.
func pkcePair(t *testing.T) (string, string) {
	t.Helper()
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge
}

// oauth2asTestServer bundles a fully-wired embedded OAuth 2.1 authorization
// server (server.go's NewProvider + handlers.go's handlers) behind a real
// httptest.Server, for end-to-end flow tests.
type oauth2asTestServer struct {
	ts       *httptest.Server
	store    *oauth2as.Store
	db       *pgxpool.Pool
	provider fosite.OAuth2Provider
	userID   string
}

func newOAuth2asTestServer(t *testing.T) *oauth2asTestServer {
	t.Helper()
	store, db := newTestStore(t)
	cfg := testhelper.NewTestConfig()
	provider := oauth2as.NewProvider(cfg, store)
	userID := uuid.NewString()

	resolveUser := func(_ *http.Request) (string, bool) {
		return userID, true
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/oauth2/authorize", oauth2as.AuthorizeHandler(provider, cfg, resolveUser),
	)
	mux.HandleFunc("/oauth2/token", oauth2as.TokenHandler(provider))
	mux.HandleFunc("/oauth2/register", oauth2as.RegisterHandler(store))
	mux.HandleFunc("/oauth2/consent-info", oauth2as.ConsentInfoHandler(store))

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &oauth2asTestServer{
		ts: ts, store: store, db: db, provider: provider, userID: userID,
	}
}

// testClientRedirectURI is the redirect_uri every test client in this file
// registers with — a loopback address, exercising validateRedirectURI's
// dev-loopback exception (clients.go), and never actually dialed since these
// tests inspect the redirect Location header directly instead of following
// it.
const testClientRedirectURI = "http://localhost:9999/callback"

func (s *oauth2asTestServer) registerClient(t *testing.T) *fosite.DefaultClient {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"redirect_uris": []string{testClientRedirectURI},
		"client_name":   "e2e test client",
	})
	require.NoError(t, err)

	resp, err := http.Post(
		s.ts.URL+"/oauth2/register",
		"application/json",
		strings.NewReader(string(body)),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var out struct {
		ClientID                string   `json:"client_id"`
		RedirectURIs            []string `json:"redirect_uris"`
		TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
		GrantTypes              []string `json:"grant_types"`
		ResponseTypes           []string `json:"response_types"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, "none", out.TokenEndpointAuthMethod)
	assert.NotEmpty(t, out.ClientID)

	//nolint:exhaustruct //Secret/RotatedSecrets/Audience are unused by these tests
	return &fosite.DefaultClient{
		ID:            out.ClientID,
		RedirectURIs:  out.RedirectURIs,
		GrantTypes:    out.GrantTypes,
		ResponseTypes: out.ResponseTypes,
		Scopes:        []string{"offline_access"},
		Public:        true,
	}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// authorizeAndGetCode drives the /oauth2/authorize leg (no-consent redirect,
// then consent=allow) and returns the authorization code from the final
// redirect to the client's own redirect_uri.
func (s *oauth2asTestServer) authorizeAndGetCode(
	t *testing.T, client *fosite.DefaultClient, challenge, state string,
) string {
	t.Helper()
	client2 := noRedirectClient()

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"offline_access"},
		"state":                 {state},
	}

	// First hit: no consent decision yet -> redirected to the web consent
	// page, carrying the same query string through.
	resp, err := client2.Get(s.ts.URL + "/oauth2/authorize?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(loc, "http://localhost:3000/oauth/consent?"))
	require.Contains(t, loc, "client_id="+client.ID)

	// Second hit: consent=allow completes the flow.
	q.Set("consent", "allow")
	resp2, err := client2.Get(s.ts.URL + "/oauth2/authorize?" + q.Encode())
	require.NoError(t, err)
	defer resp2.Body.Close()
	// fosite's own WriteAuthorizeResponse (unlike this handler's own
	// no-consent redirect above) uses 303 See Other, not 302 Found.
	require.Equal(t, http.StatusSeeOther, resp2.StatusCode)

	redirectLoc, err := url.Parse(resp2.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, state, redirectLoc.Query().Get("state"))
	code := redirectLoc.Query().Get("code")
	require.NotEmpty(t, code)
	return code
}

func (s *oauth2asTestServer) exchangeToken(
	t *testing.T, form url.Values,
) (*http.Response, tokenResponse) {
	t.Helper()
	resp, err := http.PostForm(s.ts.URL+"/oauth2/token", form)
	require.NoError(t, err)
	defer resp.Body.Close()

	var out tokenResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestOAuth2Flow_AuthorizationCodePKCE_RefreshAndFailure(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	_, challenge := pkcePair(t)
	code := srv.authorizeAndGetCode(t, client, challenge, "xyz-state-12345")

	// Wrong PKCE verifier must be rejected.
	badResp, badOut := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {"totally-the-wrong-verifier-0123456789"},
	})
	assert.NotEqual(t, http.StatusOK, badResp.StatusCode)
	assert.Empty(t, badOut.AccessToken)

	// A fresh authorization code is needed: the failed exchange above didn't
	// consume the code, but request it again from scratch for isolation.
	verifier2, challenge2 := pkcePair(t)
	code2 := srv.authorizeAndGetCode(t, client, challenge2, "state-2-abcdefgh")

	// Exchange with the correct PKCE verifier succeeds.
	resp, out := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code2},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {verifier2},
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, out.AccessToken)
	require.NotEmpty(t, out.RefreshToken)
	assert.Equal(t, "bearer", strings.ToLower(out.TokenType))

	// The resolver correctly maps the fosite-issued access token back to
	// the user ID granted at consent time.
	resolver := oauth2as.NewTokenResolver(srv.provider)
	gotUserID, err := resolver.ResolveAccessToken(context.Background(), out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, srv.userID, gotUserID)

	// The refresh token grant issues a new access token (and rotates the
	// refresh token).
	refreshResp, refreshOut := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
	})
	require.Equal(t, http.StatusOK, refreshResp.StatusCode)
	require.NotEmpty(t, refreshOut.AccessToken)
	require.NotEmpty(t, refreshOut.RefreshToken)
	assert.NotEqual(t, out.AccessToken, refreshOut.AccessToken)
	assert.NotEqual(t, out.RefreshToken, refreshOut.RefreshToken)

	gotUserID2, err := resolver.ResolveAccessToken(
		context.Background(), refreshOut.AccessToken,
	)
	require.NoError(t, err)
	assert.Equal(t, srv.userID, gotUserID2)

	// Reused immediately, the rotated-out (old) refresh token is still
	// accepted — within the reuse grace period this is treated as a
	// legitimate retry rather than theft (see storage.go's
	// refreshTokenReuseGracePeriod).
	graceResp, graceOut := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
	})
	require.Equal(t, http.StatusOK, graceResp.StatusCode)
	require.NotEmpty(t, graceOut.RefreshToken)

	// Past the grace period, reuse of a rotated-out refresh token is
	// rejected and revokes the whole token family. fosite's HMAC token
	// strategy formats tokens as "<id>.<signature>" (hmacsha.go's own
	// Signature method), so the DB row's signature is the part after the
	// dot — no need to reach into the provider to recompute it.
	tokenParts := strings.SplitN(out.RefreshToken, ".", 2)
	require.Len(t, tokenParts, 2, "fosite HMAC token must be id.signature")
	_, err = srv.db.Exec(context.Background(), `
		UPDATE auth.oauth2_refresh_tokens
		SET rotated_at = now() - interval '1 minute'
		WHERE signature = $1
	`, tokenParts[1])
	require.NoError(t, err)

	staleResp, _ := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
	})
	assert.NotEqual(t, http.StatusOK, staleResp.StatusCode)
}

func TestOAuth2Flow_MissingPKCEVerifier(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	_, challenge := pkcePair(t)
	code := srv.authorizeAndGetCode(t, client, challenge, "state-3-abcdefgh")

	resp, out := srv.exchangeToken(t, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {client.RedirectURIs[0]},
		"client_id":    {client.ID},
		// no code_verifier at all
	})
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, out.AccessToken)
}

func TestOAuth2Flow_ConsentDeny(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)
	_, challenge := pkcePair(t)

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {"offline_access"},
		"state":                 {"deny-state-abcdefgh"},
		"consent":               {"deny"},
	}

	resp, err := noRedirectClient().Get(srv.ts.URL + "/oauth2/authorize?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()
	// fosite's own WriteAuthorizeError uses 303 See Other, not 302 Found.
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "access_denied", loc.Query().Get("error"))
}

func TestRegisterHandler_InvalidRedirectURIs(t *testing.T) {
	srv := newOAuth2asTestServer(t)

	body, err := json.Marshal(map[string]any{
		"redirect_uris": []string{},
		"client_name":   "bad client",
	})
	require.NoError(t, err)

	resp, err := http.Post(
		srv.ts.URL+"/oauth2/register",
		"application/json",
		strings.NewReader(string(body)),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegisterHandler_InvalidBody(t *testing.T) {
	srv := newOAuth2asTestServer(t)

	resp, err := http.Post(
		srv.ts.URL+"/oauth2/register",
		"application/json",
		strings.NewReader("not json"),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestConsentInfoHandler(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)

	resp, err := http.Get(
		srv.ts.URL + "/oauth2/consent-info?client_id=" + client.ID + "&scope=offline_access",
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, "e2e test client", out["client_name"])
	assert.Equal(t, "offline_access", out["scope"])
	assert.Equal(t, client.ID, out["client_id"])
}

func TestConsentInfoHandler_UnknownClient(t *testing.T) {
	srv := newOAuth2asTestServer(t)

	resp, err := http.Get(srv.ts.URL + "/oauth2/consent-info?client_id=does-not-exist")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
