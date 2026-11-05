package runner_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/factory-runner/internal/runner"
)

// authServer is a stand-in OAuth server: metadata, then client_credentials tokens.
type authServer struct {
	srv       *httptest.Server
	discovers int
	mints     int
	expiresIn int
	status    int
	token     string
	user      string
	pass      string
}

func newAuthServer(t *testing.T) *authServer {
	t.Helper()
	a := &authServer{
		srv: nil, discovers: 0, mints: 0, expiresIn: 3600, status: http.StatusOK,
		token: "tok-1", user: "", pass: "",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server",
		func(w http.ResponseWriter, _ *http.Request) {
			a.discovers++
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token_endpoint": a.srv.URL + "/token",
			})
		})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		a.mints++
		a.user, a.pass, _ = r.BasicAuth()
		assert.Equal(t, "client_credentials", r.FormValue("grant_type"))
		if a.status != http.StatusOK {
			http.Error(w, "client-secret-leak-attempt", a.status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": a.token, "expires_in": a.expiresIn,
		})
	})
	a.srv = httptest.NewServer(mux)
	t.Cleanup(a.srv.Close)
	return a
}

func (a *authServer) source(now func() time.Time) *runner.MCPTokenSource {
	return &runner.MCPTokenSource{
		ServerURL: a.srv.URL, ClientID: "agent-1", ClientSecret: "s3cret",
		HTTP: nil, Now: now,
	}
}

func TestTheJobsTokenIsMintedOnceAndReusedUntilNearExpiry(t *testing.T) {
	a := newAuthServer(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	src := a.source(func() time.Time { return now })

	for range 3 {
		tok, err := src.Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "tok-1", tok)
	}
	assert.Equal(t, 1, a.mints)
	assert.Equal(t, 1, a.discovers)
	assert.Equal(t, "agent-1", a.user)
	assert.Equal(t, "s3cret", a.pass)

	// Within the margin of expiry a new one is minted; discovery is not repeated.
	now = now.Add(time.Hour - 30*time.Second)
	a.token = "tok-2"
	tok, err := src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-2", tok)
	assert.Equal(t, 2, a.mints)
	assert.Equal(t, 1, a.discovers)
}

func TestATokenWithoutALifetimeIsRefreshedAfterFiveMinutes(t *testing.T) {
	a := newAuthServer(t)
	a.expiresIn = 0
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	src := a.source(func() time.Time { return now })

	_, err := src.Token(context.Background())
	require.NoError(t, err)
	now = now.Add(3 * time.Minute)
	_, err = src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, a.mints)
	now = now.Add(2 * time.Minute)
	_, err = src.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, a.mints)
}

func TestAFailedMintReportsOnlyTheStatus(t *testing.T) {
	a := newAuthServer(t)
	a.status = http.StatusUnauthorized
	_, err := a.source(nil).Token(context.Background())
	require.ErrorContains(t, err, "status 401")
	assert.NotContains(t, err.Error(), "leak")
	assert.NotContains(t, err.Error(), "s3cret")
}

func TestAJobWithoutABoardAccountCannotGetAToken(t *testing.T) {
	a := newAuthServer(t)
	src := a.source(nil)
	src.ClientID = ""
	_, err := src.Token(context.Background())
	require.ErrorContains(t, err, "no board account")
	assert.Zero(t, a.discovers)
}

func TestBrokenDiscoveryAndTokenAnswersAreErrors(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"no metadata": func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, nil)
		},
		"no endpoint": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		},
		"not json": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`<html>`))
		},
	} {
		srv := httptest.NewServer(handler)
		src := &runner.MCPTokenSource{
			ServerURL: srv.URL, ClientID: "a", ClientSecret: "b", HTTP: nil, Now: nil,
		}
		_, err := src.Token(context.Background())
		require.ErrorContains(t, err, "token discovery", name)
		srv.Close()
	}

	// The endpoint answers 200 with no token.
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/oauth-authorization-server",
		func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"token_endpoint": srv.URL + "/t"})
		})
	mux.HandleFunc("/t", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":""}`))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()
	src := &runner.MCPTokenSource{
		ServerURL: srv.URL, ClientID: "a", ClientSecret: "b", HTTP: nil, Now: nil,
	}
	_, err := src.Token(context.Background())
	require.ErrorContains(t, err, "sent none")
}

func TestAnUnreachableServerIsAnError(t *testing.T) {
	src := &runner.MCPTokenSource{
		ServerURL: "http://127.0.0.1:1", ClientID: "a", ClientSecret: "b",
		HTTP: nil, Now: nil,
	}
	_, err := src.Token(context.Background())
	require.ErrorContains(t, err, "token discovery")
}
