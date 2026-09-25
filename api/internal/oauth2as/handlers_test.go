package oauth2as_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
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

// noRedirectClient never follows redirects, so tests can read Location.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

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

// oauth2asTestServer is the fully wired AS behind an httptest.Server.
type oauth2asTestServer struct {
	ts       *httptest.Server
	store    *oauth2as.Store
	db       *pgxpool.Pool
	provider fosite.OAuth2Provider
	key      *rsa.PrivateKey
	userID   string
	logs     *recordCapture
}

func newOAuth2asTestServer(t *testing.T) *oauth2asTestServer {
	t.Helper()
	store, db := newTestStore(t)
	cfg := testhelper.NewTestConfig()
	key := testOIDCKey(t)
	provider := oauth2as.NewProvider(cfg, store, key)
	userID := uuid.NewString()

	resolveUser := func(_ *http.Request) (oauth2as.ResolvedUser, bool) {
		return oauth2as.ResolvedUser{
			ID:          userID,
			Email:       "e2e-user@example.com",
			DisplayName: "E2E User",
			IsAdmin:     true,
		}, true
	}

	logs := newRecordCapture()
	logger := slog.New(logs)

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/oauth2/authorize",
		oauth2as.AuthorizeHandler(
			provider, cfg, resolveUser, oauth2as.OIDCKeyID(key), logger,
		),
	)
	mux.HandleFunc("/oauth2/token", oauth2as.TokenHandler(provider, logger))
	mux.HandleFunc("/oauth2/register", oauth2as.RegisterHandler(store, logger))
	mux.HandleFunc("/oauth2/consent-info", oauth2as.ConsentInfoHandler(store))
	mux.HandleFunc("/oauth2/jwks", oauth2as.JWKSHandler(key))

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &oauth2asTestServer{
		ts: ts, store: store, db: db, provider: provider, key: key,
		userID: userID, logs: logs,
	}
}

// testClientRedirectURI is loopback (validateRedirectURI's dev exception) and
// never dialed.
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
		Scope                   string   `json:"scope"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, "none", out.TokenEndpointAuthMethod)
	assert.NotEmpty(t, out.ClientID)
	assert.Equal(t, "offline_access", out.Scope)

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

// authorizeAndGetCode drives the authorize leg and returns the code.
func (s *oauth2asTestServer) authorizeAndGetCode(
	t *testing.T, client *fosite.DefaultClient, challenge, state string,
) string {
	t.Helper()
	return s.authorizeAndGetCodeWithScope(
		t, client, challenge, state, "offline_access",
	)
}

// authorizeAndGetCodeWithScope; scope == "" mimics an MCP client sending none.
func (s *oauth2asTestServer) authorizeAndGetCodeWithScope(
	t *testing.T, client *fosite.DefaultClient, challenge, state, scope string,
) string {
	t.Helper()
	client2 := noRedirectClient()

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	if scope != "" {
		q.Set("scope", scope)
	}

	resp, err := client2.Get(s.ts.URL + "/oauth2/authorize?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(loc, "http://localhost:3000/oauth/consent?"))
	require.Contains(t, loc, "client_id="+client.ID)

	q.Set("consent", "allow")
	resp2, err := client2.Get(s.ts.URL + "/oauth2/authorize?" + q.Encode())
	require.NoError(t, err)
	defer resp2.Body.Close()
	// fosite's WriteAuthorizeResponse uses 303, not 302.
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

	badResp, badOut := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {"totally-the-wrong-verifier-0123456789"},
	})
	assert.NotEqual(t, http.StatusOK, badResp.StatusCode)
	assert.Empty(t, badOut.AccessToken)

	verifier2, challenge2 := pkcePair(t)
	code2 := srv.authorizeAndGetCode(t, client, challenge2, "state-2-abcdefgh")

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

	resolver := oauth2as.NewTokenResolver(srv.provider)
	gotUserID, err := resolver.ResolveAccessToken(context.Background(), out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, srv.userID, gotUserID)

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

	// Within refreshTokenReuseGracePeriod, reuse is treated as a retry.
	graceResp, graceOut := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
	})
	require.Equal(t, http.StatusOK, graceResp.StatusCode)
	require.NotEmpty(t, graceOut.RefreshToken)

	// Past the grace period, reuse revokes the whole token family. HMAC tokens are
	// "<id>.<signature>", so the DB signature is the part after the dot.
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
	// fosite's WriteAuthorizeError uses 303, not 302.
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
