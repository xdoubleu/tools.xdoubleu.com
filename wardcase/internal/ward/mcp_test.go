package ward_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/wardcase/internal/ward"
	"tools.xdoubleu.com/wardcase/vm"
)

// fakeMCP is an MCP server and its authorization server in one: discovery,
// registration, authorization code with PKCE, refresh with rotation.
type fakeMCP struct {
	srv *httptest.Server

	mu        sync.Mutex
	redirect  string
	challenge string
	resource  string
	access    string
	refresh   string
	refreshes int
	issued    int
	errs      []string
}

func newFakeMCP(t *testing.T) *fakeMCP {
	t.Helper()
	f := &fakeMCP{} //nolint:exhaustruct // state starts empty
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", f.mcp)
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", f.resourceMeta)
	mux.HandleFunc("/.well-known/oauth-authorization-server", f.serverMeta)
	mux.HandleFunc("/register", f.register)
	mux.HandleFunc("/authorize", f.authorize)
	mux.HandleFunc("/token", f.token)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeMCP) resourceMeta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"resource": f.srv.URL + "/mcp", "authorization_servers": []string{f.srv.URL},
		"scopes_supported": []string{"read"},
	})
}

func (f *fakeMCP) serverMeta(w http.ResponseWriter, _ *http.Request) {
	base := f.srv.URL
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer": base, "authorization_endpoint": base + "/authorize",
		"token_endpoint": base + "/token", "registration_endpoint": base + "/register",
		"code_challenge_methods_supported": []string{"S256"},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeMCP) fail(msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, msg)
}

func (f *fakeMCP) mcp(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	want := "Bearer " + f.access
	f.mu.Unlock()
	if f.access == "" || r.Header.Get("Authorization") != want {
		w.Header().Set("WWW-Authenticate",
			`Bearer resource_metadata="`+f.srv.URL+`/.well-known/oauth-protected-resource/mcp"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func (f *fakeMCP) register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RedirectURIs []string `json:"redirect_uris"`
		AuthMethod   string   `json:"token_endpoint_auth_method"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.RedirectURIs) != 1 || body.AuthMethod != "none" {
		f.fail("bad registration")
	}
	f.mu.Lock()
	f.redirect = body.RedirectURIs[0]
	f.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"client_id": "client-1"})
}

func (f *fakeMCP) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.mu.Lock()
	defer f.mu.Unlock()
	if q.Get("response_type") != "code" || q.Get("client_id") != "client-1" ||
		q.Get("redirect_uri") != f.redirect || q.Get("code_challenge_method") != "S256" ||
		q.Get("resource") != f.srv.URL+"/mcp" || q.Get("scope") != "read" {
		f.errs = append(f.errs, "bad authorize request: "+r.URL.RawQuery)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	f.challenge, f.resource = q.Get("code_challenge"), q.Get("resource")
	target, _ := url.Parse(f.redirect)
	v := target.Query()
	v.Set("code", "code-1")
	v.Set("state", q.Get("state"))
	target.RawQuery = v.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (f *fakeMCP) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "code-1" ||
			base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge ||
			r.Form.Get("resource") != f.resource {
			f.errs = append(f.errs, "bad code exchange")
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
			return
		}
	case "refresh_token":
		f.refreshes++
		if r.Form.Get("refresh_token") != f.refresh || r.Form.Get("client_id") != "client-1" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
			return
		}
	}
	f.issued++
	f.access = "access-" + string(rune('0'+f.issued))
	f.refresh = "refresh-" + string(rune('0'+f.issued))
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": f.access, "token_type": "Bearer", "expires_in": 3600,
		"refresh_token": f.refresh,
	})
}

// browser follows the sign-in URL the way a user's browser would.
func browser(t *testing.T) func(string) error {
	t.Helper()
	return func(target string) error {
		go func() {
			resp, err := http.Get(target)
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
}

func mcpConfig(f *fakeMCP, jobs bool) ward.Config {
	return ward.Config{
		Order: []string{"claude"}, Overflow: ward.OverflowOff,
		OpenCode:    ward.OpenCodeConfig{Model: ""},
		MCP:         []ward.MCPConfig{{Name: "fake", URL: f.srv.URL + "/mcp", Jobs: jobs}},
		Marketplace: nil,
	}
}

func login(t *testing.T, f *fakeMCP, store ward.Secrets) {
	t.Helper()
	var out strings.Builder
	err := ward.LoginMCP(context.Background(), mcpConfig(f, false), store,
		ward.MCPLogin{HTTP: nil, Open: browser(t), Out: &out}, "fake")
	require.NoError(t, err)
	assert.Contains(t, out.String(), "/authorize?")
	assert.Empty(t, f.errs)
}

func TestLoginRunsTheMCPAuthorizationFlowAndKeepsOnlyTheRefreshToken(t *testing.T) {
	f := newFakeMCP(t)
	store := memSecrets{}

	login(t, f, store)

	var creds map[string]string
	require.NoError(t, json.Unmarshal([]byte(store["mcp-fake"]), &creds))
	assert.Equal(t, "client-1", creds["client_id"])
	assert.Equal(t, "refresh-1", creds["refresh_token"])
	assert.Equal(t, f.srv.URL+"/mcp", creds["resource"])
	assert.NotContains(t, store["mcp-fake"], "access-1")
	signedIn, err := ward.MCPSignedIn(store, "fake")
	require.NoError(t, err)
	assert.True(t, signedIn)
}

func TestLoginRefusesAServerWithoutDiscoveryOrAnUnknownName(t *testing.T) {
	f := newFakeMCP(t)
	plain := httptest.NewServer(http.NotFoundHandler())
	defer plain.Close()
	cfg := ward.Config{
		Order: nil, Overflow: "", OpenCode: ward.OpenCodeConfig{Model: ""},
		MCP: []ward.MCPConfig{{Name: "plain", URL: plain.URL + "/mcp", Jobs: false}},
	}

	err := ward.LoginMCP(context.Background(), cfg, memSecrets{},
		ward.MCPLogin{HTTP: nil, Open: nil, Out: &strings.Builder{}}, "plain")
	require.ErrorContains(t, err, "RFC 9728")
	err = ward.LoginMCP(context.Background(), mcpConfig(f, false), memSecrets{},
		ward.MCPLogin{HTTP: nil, Open: nil, Out: &strings.Builder{}}, "nope")
	require.ErrorContains(t, err, `no [[mcp]] named "nope"`)
}

func TestLoginStopsWhenTheCallbackCarriesAnError(t *testing.T) {
	f := newFakeMCP(t)
	open := func(target string) error {
		u, _ := url.Parse(target)
		go func() {
			q := u.Query()
			cb := q.Get("redirect_uri") + "?error=access_denied&state=" + q.Get("state")
			if resp, err := http.Get(cb); err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
	err := ward.LoginMCP(context.Background(), mcpConfig(f, false), memSecrets{},
		ward.MCPLogin{HTTP: nil, Open: open, Out: &strings.Builder{}}, "fake")
	require.ErrorContains(t, err, "access_denied")
}

// proxyConfig runs one session and returns the egress config the proxy was built
// from, and the VM's environment.
func proxyConfig(
	t *testing.T, f *fixture, headless bool,
) (vm.EgressConfig, map[string]string) {
	t.Helper()
	var got vm.EgressConfig
	f.r.NewProxy = func(cfg vm.EgressConfig) (vm.ProxyListener, error) {
		got = cfg
		return vm.NewEgressProxy(cfg)
	}
	_, err := f.r.Run(context.Background(), ward.Request{
		Dir: "/w", Harness: "claude", Args: []string{"p"}, Print: headless, InPlace: true,
	})
	require.NoError(t, err)
	return got, f.box.runs[len(f.box.runs)-1].spec.Env
}

func mcpFixture(t *testing.T, f *fakeMCP, jobs bool) (*fixture, memSecrets) {
	t.Helper()
	fx := newFixture(t, ward.OverflowOff, false)
	store, ok := fx.r.Secrets.(memSecrets)
	require.True(t, ok)
	fx.r.Config = mcpConfig(f, jobs)
	return fx, store
}

func TestASignedInServerGetsAProxyRouteAndTheVMOnlyItsName(t *testing.T) {
	f := newFakeMCP(t)
	fx, store := mcpFixture(t, f, false)
	login(t, f, store)

	cfg, env := proxyConfig(t, fx, false)

	require.Len(t, cfg.MCPServers, 1)
	assert.Equal(t, "fake", cfg.MCPServers[0].Name)
	assert.Equal(t, f.srv.URL+"/mcp", cfg.MCPServers[0].URL)
	assert.Equal(t, "fake", env[vm.MCPServersEnv])
	assert.Equal(t, vm.MCPOff, env[vm.MCPEnv], "the board stays off")
	token, err := cfg.MCPServers[0].Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "access-2", token, "the refresh token bought an access token")
	for k, v := range env {
		assert.NotContains(t, v, "access-", k)
		assert.NotContains(t, v, "refresh-", k)
	}
	assert.Empty(t, fx.notes.String())
}

func TestTheAccessTokenIsReusedUntilRejectedThenRefreshedAndRotated(t *testing.T) {
	f := newFakeMCP(t)
	fx, store := mcpFixture(t, f, false)
	login(t, f, store)
	cfg, _ := proxyConfig(t, fx, false)
	srv := cfg.MCPServers[0]
	ctx := context.Background()

	first, err := srv.Token(ctx)
	require.NoError(t, err)
	again, err := srv.Token(ctx)
	require.NoError(t, err)
	assert.Equal(t, first, again)
	assert.Equal(t, 1, f.refreshes, "one refresh serves both calls")
	assert.Contains(t, store["mcp-fake"], `"refresh-2"`, "rotated token is kept")

	srv.Rejected()
	next, err := srv.Token(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, first, next)
	assert.Equal(t, 2, f.refreshes)
	assert.Contains(t, store["mcp-fake"], `"refresh-3"`)
}

func TestARevokedSignInSaysToLogInAgain(t *testing.T) {
	f := newFakeMCP(t)
	fx, store := mcpFixture(t, f, false)
	login(t, f, store)
	store["mcp-fake"] = strings.Replace(store["mcp-fake"], "refresh-1", "stale", 1)
	cfg, _ := proxyConfig(t, fx, false)

	_, err := cfg.MCPServers[0].Token(context.Background())

	require.ErrorContains(t, err, "wardcase mcp login fake")
}

func TestAServerThatIsNotSignedInGetsNoRouteAndTheSessionSaysSo(t *testing.T) {
	f := newFakeMCP(t)
	fx, _ := mcpFixture(t, f, false)

	cfg, env := proxyConfig(t, fx, false)

	assert.Empty(t, cfg.MCPServers)
	assert.NotContains(t, env, vm.MCPServersEnv)
	assert.Contains(t, fx.notes.String(),
		"MCP server fake is not signed in (wardcase mcp login fake)")
}

func TestHeadlessRunsGetOnlyTheServersMarkedForJobs(t *testing.T) {
	f := newFakeMCP(t)
	fx, store := mcpFixture(t, f, false)
	login(t, f, store)
	cfg, _ := proxyConfig(t, fx, true)
	assert.Empty(t, cfg.MCPServers)

	fx, store = mcpFixture(t, f, true)
	login(t, f, store)
	cfg, _ = proxyConfig(t, fx, true)
	assert.Len(t, cfg.MCPServers, 1)
}

func TestLogoutForgetsTheSignIn(t *testing.T) {
	f := newFakeMCP(t)
	store := memSecrets{}
	login(t, f, store)

	require.NoError(t, ward.LogoutMCP(store, "fake"))

	signedIn, err := ward.MCPSignedIn(store, "fake")
	require.NoError(t, err)
	assert.False(t, signedIn)
}

func TestAnUnreadableStoredSignInCountsAsNone(t *testing.T) {
	store := memSecrets{"mcp-x": "not json"}
	signedIn, err := ward.MCPSignedIn(store, "x")
	require.NoError(t, err)
	assert.False(t, signedIn)
}

type failingSecrets struct{ memSecrets }

func (f failingSecrets) Get(name string) (string, error) {
	if strings.HasPrefix(name, "mcp-") {
		return "", errors.New("keychain locked")
	}
	return f.memSecrets.Get(name)
}

func TestAKeychainFailureStopsTheRunBeforeTheVM(t *testing.T) {
	f := newFakeMCP(t)
	fx, _ := mcpFixture(t, f, false)
	fx.r.Secrets = failingSecrets{memSecrets{ward.SecretAnthropic: "oat"}}

	_, err := fx.r.Run(context.Background(), ward.Request{
		Dir: "/w", Harness: "claude", Args: nil, Print: false, InPlace: true,
	})

	require.Error(t, err)
	assert.Empty(t, fx.box.runs)
}
