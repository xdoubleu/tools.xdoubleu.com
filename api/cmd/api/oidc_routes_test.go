package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/platform/auth"
)

// With OIDC sign-in on, the login routes exist and an authorize request
// without a session is sent through them.
func TestMux_OIDCSignIn(t *testing.T) {
	//nolint:exhaustruct // only the OIDC settings matter
	provider, err := auth.NewOIDCProvider(config.Config{
		OIDCIssuer: "http://127.0.0.1:1", OIDCClientID: "c",
		OIDCClientSecret: "s", JWTSecret: "secret", APIURL: "http://api.test",
	})
	require.NoError(t, err)
	testApp.auth.OIDC = provider
	t.Cleanup(func() { testApp.auth.OIDC = nil })
	mux := testApp.mux()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet, "/oauth2/authorize?client_id=c&state=s", nil,
	))
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(
		t,
		auth.OIDCLoginPath+"?redirect=%2Foauth2%2Fauthorize%3Fclient_id%3Dc%26state%3Ds",
		rec.Header().Get("Location"),
	)

	// The issuer is unreachable, so login reports it rather than redirecting.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, auth.OIDCLoginPath, nil))
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet, auth.OIDCCallbackPath+"?error=access_denied", nil,
	))
	assert.Equal(t, http.StatusFound, rec.Code)
}

func TestNewOIDCProvider(t *testing.T) {
	//nolint:exhaustruct // only the OIDC settings matter
	assert.Nil(t, newOIDCProvider(config.Config{OIDCSignIn: false}))

	assert.Panics(t, func() {
		//nolint:exhaustruct // only the OIDC settings matter
		newOIDCProvider(config.Config{OIDCSignIn: true})
	}, "on without an issuer and client")

	//nolint:exhaustruct // only the OIDC settings matter
	provider := newOIDCProvider(config.Config{
		OIDCSignIn: true, OIDCIssuer: "https://id.test", OIDCClientID: "c",
		OIDCClientSecret: "s", JWTSecret: "secret",
	})
	assert.NotNil(t, provider)
}

func TestAccountURL(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	assert.Empty(t, accountURL(cfg))

	cfg.OIDCSignIn, cfg.OIDCIssuer = true, "https://id.example.com/"
	assert.Equal(t, "https://id.example.com/ui/console/users/me", accountURL(cfg))
}

func TestNewDirectory(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	assert.Nil(t, newDirectory(cfg))

	cfg.OIDCIssuer, cfg.ZitadelServiceToken = "https://id.example.com", "pat"
	assert.NotNil(t, newDirectory(cfg))
}
