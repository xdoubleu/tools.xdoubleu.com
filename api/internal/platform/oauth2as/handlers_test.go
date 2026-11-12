package oauth2as_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	fosite "authelia.com/provider/oauth2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/platform/oauth2as"
	"tools.xdoubleu.com/internal/kit/testhelper"
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
	provider fosite.Provider
	key      *rsa.PrivateKey
	userID   string
	logs     *recordCapture
	// nonAdmin makes the signed-in user a person without the admin role.
	nonAdmin  bool
	delegator *fakeDelegator
}

// fakeDelegator delegates to agentUserID, or fails with err.
type fakeDelegator struct {
	agentUserID  string
	err          error
	delegated    []string
	undelegated  []string
	undelegateEr error
}

func (f *fakeDelegator) Delegate(
	_ context.Context, _ oauth2as.ResolvedUser, clientID, accountID string,
) (oauth2as.ResolvedUser, error) {
	if f.err != nil {
		return oauth2as.ResolvedUser{}, f.err
	}
	f.delegated = append(f.delegated, clientID+"="+accountID)
	return oauth2as.ResolvedUser{
		ID: f.agentUserID, Email: "agent@agent.invalid", DisplayName: "", IsAdmin: false,
	}, nil
}

func (f *fakeDelegator) Undelegate(_ context.Context, clientID string) error {
	f.undelegated = append(f.undelegated, clientID)
	return f.undelegateEr
}

func newOAuth2asTestServer(t *testing.T) *oauth2asTestServer {
	t.Helper()
	return newOAuth2asTestServerWithConfig(t, testhelper.NewTestConfig())
}

func newOAuth2asTestServerWithConfig(
	t *testing.T, cfg config.Config,
) *oauth2asTestServer {
	t.Helper()
	store, db := newTestStore(t)
	key := testOIDCKey(t)
	provider := oauth2as.NewProvider(cfg, store, key)
	userID := uuid.NewString()
	//nolint:exhaustruct // ts and logs are set once the mux is built
	srv := &oauth2asTestServer{
		store: store, db: db, provider: provider, key: key, userID: userID,
		nonAdmin: false,
		//nolint:exhaustruct // a fake that delegates until a test sets err
		delegator: &fakeDelegator{agentUserID: uuid.NewString()},
	}

	resolveUser := func(_ *http.Request) (oauth2as.ResolvedUser, bool) {
		return oauth2as.ResolvedUser{
			ID:          userID,
			Email:       "e2e-user@example.com",
			DisplayName: "E2E User",
			IsAdmin:     !srv.nonAdmin,
		}, true
	}

	logs := newRecordCapture()
	logger := slog.New(logs)

	mux := http.NewServeMux()
	mux.HandleFunc(
		"/oauth2/authorize",
		oauth2as.AuthorizeHandler(
			provider, cfg, resolveUser, srv.delegator, oauth2as.OIDCKeyID(key),
			logger,
		),
	)
	mux.HandleFunc(
		"/oauth2/token",
		oauth2as.TokenHandler(provider, oauth2as.NewMachineSubjects(nil), logger),
	)
	mux.HandleFunc("/oauth2/register", oauth2as.RegisterHandler(store, logger))
	mux.HandleFunc(
		"/oauth2/consent-info",
		oauth2as.ConsentInfoHandler(store, cfg, resolveUser),
	)
	mux.HandleFunc("/oauth2/jwks", oauth2as.JWKSHandler(key))

	srv.ts = httptest.NewServer(mux)
	t.Cleanup(srv.ts.Close)
	srv.logs = logs
	return srv
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
	// RFC 7591: the echoed scope tells the client it may request offline_access
	// (refresh tokens) and openid (a minimal ID token).
	assert.Equal(t, "offline_access openid", out.Scope)

	//nolint:exhaustruct //secrets, audience and binding are unused by these tests
	return &fosite.DefaultClient{
		ID:            out.ClientID,
		RedirectURIs:  out.RedirectURIs,
		GrantTypes:    out.GrantTypes,
		ResponseTypes: out.ResponseTypes,
		Scopes:        []string{"offline_access", "openid"},
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
	extra := url.Values{}
	if scope != "" {
		extra.Set("scope", scope)
	}
	return s.authorizeAndGetCodeWithParams(t, client, challenge, state, extra)
}

// authorizeAndGetCodeWithParams adds extra to the authorize query.
func (s *oauth2asTestServer) authorizeAndGetCodeWithParams(
	t *testing.T, client *fosite.DefaultClient, challenge, state string,
	extra url.Values,
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
	for k, v := range extra {
		q[k] = v
	}

	resp, err := client2.Get(s.ts.URL + "/oauth2/authorize?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	require.True(t, strings.HasPrefix(loc, "http://localhost:3000/oauth/consent?"))
	require.Contains(t, loc, "client_id="+client.ID)

	token := s.consentToken(t, q)
	q.Set("consent", "allow")
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		s.ts.URL+"/oauth2/authorize?"+q.Encode(), nil,
	)
	require.NoError(t, err)
	req.Header.Set(oauth2as.ConsentTokenHeader, token)
	resp2, err := client2.Do(req)
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

// consentToken fetches the token the consent page gets for q.
func (s *oauth2asTestServer) consentToken(t *testing.T, q url.Values) string {
	t.Helper()
	resp, err := http.Get(s.ts.URL + "/oauth2/consent-info?" + q.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var out struct {
		ConsentToken string `json:"consent_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.ConsentToken)
	return out.ConsentToken
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

// MCP clients send an RFC 8707 resource indicator on both legs.
func TestOAuth2Flow_ResourceIndicator(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	srv := newOAuth2asTestServerWithConfig(t, cfg)
	client := srv.registerClient(t)
	resource := cfg.APIURL + "/apps/mcp"

	verifier, challenge := pkcePair(t)
	code := srv.authorizeAndGetCodeWithParams(
		t, client, challenge, "resource-state-123",
		url.Values{"scope": {"offline_access"}, "resource": {resource}},
	)

	resp, out := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {verifier},
		"resource":      {resource},
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, out.AccessToken)

	refreshResp, refreshOut := srv.exchangeToken(t, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {out.RefreshToken},
		"client_id":     {client.ID},
		"resource":      {resource},
	})
	require.Equal(t, http.StatusOK, refreshResp.StatusCode)
	assert.NotEmpty(t, refreshOut.AccessToken)
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

	resp, err := noRedirectClient().Post(
		srv.ts.URL+"/oauth2/authorize?"+q.Encode(), "", nil,
	)
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

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, "e2e test client", out["client_name"])
	assert.Equal(t, "offline_access", out["scope"])
	assert.Equal(t, client.ID, out["client_id"])
	assert.Equal(t, true, out["can_delegate"])
}

func TestConsentInfoHandler_PersonCantDelegate(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	srv.nonAdmin = true
	client := srv.registerClient(t)

	resp, err := http.Get(srv.ts.URL + "/oauth2/consent-info?client_id=" + client.ID)
	require.NoError(t, err)
	defer resp.Body.Close()

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, false, out["can_delegate"])
}

// approve posts an approval of a fresh request for client with extra params
// and returns the response.
func (s *oauth2asTestServer) approve(
	t *testing.T, client *fosite.DefaultClient, challenge string, extra url.Values,
) *http.Response {
	t.Helper()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"delegation-state-1"},
	}
	token := s.consentToken(t, q)
	for k, v := range extra {
		q[k] = v
	}
	q.Set("consent", "allow")
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		s.ts.URL+"/oauth2/authorize?"+q.Encode(), nil,
	)
	require.NoError(t, err)
	req.Header.Set(oauth2as.ConsentTokenHeader, token)
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestAuthorize_DelegatedClientActsAsTheAgent(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)
	verifier, challenge := pkcePair(t)

	code := srv.authorizeAndGetCodeWithParams(t, client, challenge, "delegate-state",
		url.Values{oauth2as.AgentAccountParam: {"account-1"}})
	resp, out := srv.exchangeToken(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {client.RedirectURIs[0]},
		"client_id":     {client.ID},
		"code_verifier": {verifier},
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	subject, err := oauth2as.NewTokenResolver(srv.provider).
		ResolveAccessToken(context.Background(), out.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, srv.delegator.agentUserID, subject)
	assert.Equal(t, []string{client.ID + "=account-1"}, srv.delegator.delegated)
	assert.Empty(t, srv.delegator.undelegated)
}

func TestAuthorize_PersonalApprovalEndsDelegation(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)
	_, challenge := pkcePair(t)

	srv.authorizeAndGetCode(t, client, challenge, "personal-state")
	assert.Equal(t, []string{client.ID}, srv.delegator.undelegated)
	assert.Empty(t, srv.delegator.delegated)
}

func TestAuthorize_DelegationRefused(t *testing.T) {
	cases := map[string]struct {
		setup func(*oauth2asTestServer)
		error string
	}{
		"person": {
			setup: func(s *oauth2asTestServer) { s.nonAdmin = true },
			error: "access_denied",
		},
		"refused": {
			setup: func(s *oauth2asTestServer) { s.delegator.err = errors.New("revoked") },
			error: "invalid_request",
		},
		"undelegate fails": {
			setup: func(s *oauth2asTestServer) {
				s.delegator.undelegateEr = errors.New("db down")
			},
			error: "server_error",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := newOAuth2asTestServer(t)
			client := srv.registerClient(t)
			_, challenge := pkcePair(t)
			tc.setup(srv)

			extra := url.Values{oauth2as.AgentAccountParam: {"account-1"}}
			if name == "undelegate fails" {
				extra = nil
			}
			resp := srv.approve(t, client, challenge, extra)
			loc, err := url.Parse(resp.Header.Get("Location"))
			require.NoError(t, err)
			assert.Equal(t, tc.error, loc.Query().Get("error"))
			assert.Empty(t, loc.Query().Get("code"))
		})
	}
}

func TestAuthorize_WithoutDelegatorSignsInAsTheUser(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	store, _ := newTestStore(t)
	key := testOIDCKey(t)
	provider := oauth2as.NewProvider(cfg, store, key)
	user := oauth2as.ResolvedUser{
		ID: uuid.NewString(), Email: "", DisplayName: "", IsAdmin: true,
	}
	resolve := func(_ *http.Request) (oauth2as.ResolvedUser, bool) { return user, true }
	handler := oauth2as.AuthorizeHandler(
		provider, cfg, resolve, nil, oauth2as.OIDCKeyID(key), slog.New(newRecordCapture()),
	)
	//nolint:exhaustruct // only registerClient and approve are used
	srv := &oauth2asTestServer{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/authorize", handler)
	mux.HandleFunc("/oauth2/register", oauth2as.RegisterHandler(store, slog.Default()))
	mux.HandleFunc(
		"/oauth2/consent-info",
		oauth2as.ConsentInfoHandler(store, cfg, resolve),
	)
	srv.ts = httptest.NewServer(mux)
	t.Cleanup(srv.ts.Close)
	client := srv.registerClient(t)
	_, challenge := pkcePair(t)

	assert.Equal(t, http.StatusSeeOther, srv.approve(t, client, challenge, nil).StatusCode)
	resp := srv.approve(
		t,
		client,
		challenge,
		url.Values{oauth2as.AgentAccountParam: {"a"}},
	)
	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "access_denied", loc.Query().Get("error"))
}

func TestConsentInfoHandler_UnknownClient(t *testing.T) {
	srv := newOAuth2asTestServer(t)

	resp, err := http.Get(srv.ts.URL + "/oauth2/consent-info?client_id=does-not-exist")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// A token minted for one request can't approve another (here: another state).
func TestOAuth2Authorize_ConsentTokenBoundToRequest(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)
	_, challenge := pkcePair(t)

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"original-state-abcdefgh"},
	}
	token := srv.consentToken(t, q)

	q.Set("state", "swapped-state-abcdefgh")
	q.Set("consent", "allow")
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		srv.ts.URL+"/oauth2/authorize?"+q.Encode(), nil,
	)
	require.NoError(t, err)
	req.Header.Set(oauth2as.ConsentTokenHeader, token)
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	loc, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "request_unauthorized", loc.Query().Get("error"))
	assert.Empty(t, loc.Query().Get("code"))
}

// decideAsJSON posts a decision the way the consent page does and returns the
// response.
func (s *oauth2asTestServer) decideAsJSON(
	t *testing.T, q url.Values, token string,
) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost,
		s.ts.URL+"/oauth2/authorize?"+q.Encode(), nil,
	)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set(oauth2as.ConsentTokenHeader, token)
	}
	resp, err := noRedirectClient().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decisionQuery(client *fosite.DefaultClient, challenge, consent string) url.Values {
	return url.Values{
		"response_type":         {"code"},
		"client_id":             {client.ID},
		"redirect_uri":          {client.RedirectURIs[0]},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {"json-state-abcdefgh"},
		"consent":               {consent},
	}
}

func TestOAuth2Authorize_AcceptJSONReturnsApprovalLocation(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)
	_, challenge := pkcePair(t)
	q := decisionQuery(client, challenge, "allow")
	token := srv.consentToken(t, q)

	resp := srv.decideAsJSON(t, q, token)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Empty(t, resp.Header.Get("Location"))
	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	loc, err := url.Parse(body["location"])
	require.NoError(t, err)
	assert.Equal(t, "json-state-abcdefgh", loc.Query().Get("state"))
	assert.NotEmpty(t, loc.Query().Get("code"))
}

func TestOAuth2Authorize_AcceptJSONReturnsDenialLocation(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	client := srv.registerClient(t)
	_, challenge := pkcePair(t)

	resp := srv.decideAsJSON(t, decisionQuery(client, challenge, "deny"), "")

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	loc, err := url.Parse(body["location"])
	require.NoError(t, err)
	assert.Equal(t, "access_denied", loc.Query().Get("error"))
}

func TestOAuth2Authorize_AcceptJSONLeavesUnredirectableErrorsAlone(t *testing.T) {
	srv := newOAuth2asTestServer(t)
	_, challenge := pkcePair(t)
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {"unknown-client"},
		"redirect_uri":          {"https://evil.example/cb"},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"consent":               {"allow"},
	}

	resp := srv.decideAsJSON(t, q, "")

	assert.GreaterOrEqual(t, resp.StatusCode, http.StatusBadRequest)
	assert.Empty(t, resp.Header.Get("Location"))
}
