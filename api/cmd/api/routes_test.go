package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/mocks"
)

// TestDomainMiddleware_PathRewrite verifies that the domainMiddleware rewrites
// request paths when the Host matches a registered app domain.
// watchparty.xdoubleu.com is the only app that overrides GetDomain(), and
// GET /watchparty/api/signaling is a real route — so a rewritten request must
// land on it (non-404) while the same path without the Host rewrite 404s.
func TestDomainMiddleware_PathRewrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/signaling", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)
	rewritten := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rewritten, req)

	// Control: without the Host rewrite the same path has no route.
	noRewrite := doInProcess(t, http.MethodGet, "/api/signaling", "", "", &accessToken)

	assert.Equal(t, http.StatusNotFound, noRewrite.Code)
	assert.NotEqual(t, http.StatusNotFound, rewritten.Code)
}

// TestDomainMiddleware_RootRewrite exercises the root rewrite branch
// ("/" → "/<app>/"): the rewritten request must land on the same route as a
// direct request to the rewritten path.
func TestDomainMiddleware_RootRewrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)
	rewritten := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rewritten, req)

	direct := doInProcess(t, http.MethodGet, "/watchparty/", "", "", &accessToken)

	assert.Equal(t, direct.Code, rewritten.Code)
}

// TestAppAccess_AdminGrantedPath covers the AppAccess "granted" branch: an
// admin user reaches a grant-protected app RPC without an explicit app grant.
func TestAppAccess_AdminGrantedPath(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(
		t,
		http.MethodPost,
		"/todos.v1.TaskService/ListTasks",
		"{}",
		"application/json",
		&accessToken,
	)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

// TestRoutes_ThrottleEnabled verifies the full middleware chain (rate limiter,
// CORS, Sentry) is constructed when Throttle is true and still serves requests
// with security headers applied.
func TestRoutes_ThrottleEnabled(t *testing.T) {
	handler := throttledRoutes(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
}

// TestCORSPreflight_ConnectProtocolVersion verifies that CORS preflight
// requests allow connect-protocol-version header when Throttle is enabled.
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

// throttledRoutes builds a Routes() handler from an Application configured
// with Throttle enabled.
func throttledRoutes(t *testing.T) http.Handler {
	t.Helper()

	logger := logging.NewNopLogger()
	cfg := config.New(logger)
	cfg.Env = configtools.TestEnv
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
		mocks.NewMockedGoTrueClient(),
	)

	handler := throttledApp.Routes()
	require.NotNil(t, handler)
	return handler
}
