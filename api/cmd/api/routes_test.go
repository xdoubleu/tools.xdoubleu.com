package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/logging"
)

// TestDomainMiddleware_PathRewrite: a watchparty-domain Host rewrites onto a
// real route; without it the path 404s.
func TestDomainMiddleware_PathRewrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/signaling", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)
	rewritten := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rewritten, req)

	noRewrite := doInProcess(t, http.MethodGet, "/api/signaling", "", "", &accessToken)

	assert.Equal(t, http.StatusNotFound, noRewrite.Code)
	assert.NotEqual(t, http.StatusNotFound, rewritten.Code)
}

// TestDomainMiddleware_RootRewrite: "/" rewrites to "/<app>/".
func TestDomainMiddleware_RootRewrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)
	rewritten := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rewritten, req)

	direct := doInProcess(t, http.MethodGet, "/watchparty/", "", "", &accessToken)

	assert.Equal(t, direct.Code, rewritten.Code)
}

// TestAppAccess_AdminGrantedPath: admins pass without an explicit grant.
func TestAppAccess_AdminGrantedPath(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(
		t,
		http.MethodPost,
		"/recipes.v1.RecipesService/ListRecipes",
		"{}",
		"application/json",
		&accessToken,
	)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

// TestAppAccess_DeniedReturns403: AppAccess guards RPCs, so a denial is a
// plain 403, not a redirect a fetch() client would silently follow.
func TestAppAccess_DeniedReturns403(t *testing.T) {
	demoteToUser(t)
	revokeAppAccess(t, testUserID, "recipes")

	rr := doInProcess(
		t,
		http.MethodPost,
		"/recipes.v1.RecipesService/ListRecipes",
		"{}",
		"application/json",
		&accessToken,
	)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
}

// TestRoutes_ThrottleEnabled: the full middleware chain serves with security
// headers.
func TestRoutes_ThrottleEnabled(t *testing.T) {
	handler := throttledRoutes(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
}

// TestCORSPreflight_ConnectProtocolVersion: preflight allows
// connect-protocol-version.
func TestCORSPreflight_ConnectProtocolVersion(t *testing.T) {
	handler := throttledRoutes(t)
	rr := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "connect-protocol-version")

	handler.ServeHTTP(rr, req)

	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowHeaders, "connect-protocol-version")
}

// throttledRoutes builds Routes() with Throttle enabled.
func throttledRoutes(t *testing.T) http.Handler {
	t.Helper()

	logger := logging.NewNopLogger()
	cfg := config.New(logger)
	cfg.Env = config.TestEnv
	cfg.Throttle = true

	postgresDB, err := postgres.Connect(
		logger,
		cfg.DBDsn,
		25,
		"15m",
		5,
		15*time.Second,
		30*time.Second,
	)
	require.NoError(t, err)
	t.Cleanup(postgresDB.Close)

	throttledApp := NewApplication(
		logger,
		cfg,
		postgresDB,
	)

	handler := throttledApp.Routes()
	require.NotNil(t, handler)
	return handler
}
