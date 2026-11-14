package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
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
