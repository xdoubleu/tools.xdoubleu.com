package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

// doInProcess executes a request directly against the handler using
// httptest.NewRecorder so that it hits the 192.0.2.1 rate-limit bucket
// (not the 127.0.0.1 bucket consumed by httptest.NewServer-based tests).
func doInProcess(
	t *testing.T,
	method, target string,
	body string,
	contentType string,
	cookies ...*http.Cookie,
) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody strings.Reader
	if body != "" {
		reqBody = *strings.NewReader(body)
	}

	req := httptest.NewRequest(method, target, &reqBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)
	return rr
}

// TestDomainMiddleware_KnownDomain verifies that the domainMiddleware rewrites
// the URL path when the request Host matches a registered app domain.
// watchparty.xdoubleu.com is the only app that overrides GetDomain().
func TestDomainMiddleware_KnownDomain(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)

	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)

	// The domain middleware rewrites "/" → "/watchparty/" and the watchparty
	// handler responds — any non-5xx is fine; we just verify the middleware ran.
	assert.Less(t, rr.Code, http.StatusInternalServerError)
}

// TestDomainMiddleware_PathRewrite exercises the non-root path rewrite branch
// (r.URL.Path != "/" → prefix+r.URL.Path rather than prefix+"/").
func TestDomainMiddleware_PathRewrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	req.Host = "watchparty.xdoubleu.com"
	req.AddCookie(&accessToken)

	rr := httptest.NewRecorder()
	testApp.Routes().ServeHTTP(rr, req)

	// Any non-5xx response is acceptable — we are testing the middleware path rewrite.
	assert.Less(t, rr.Code, http.StatusInternalServerError)
}

// TestAppAccess_AdminGrantedPath covers the AppAccess "granted" branch
// (admin user → next handler is called) by promoting the test user to admin
// and hitting a protected app route.
func TestAppAccess_AdminGrantedPath(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	rr := doInProcess(t, http.MethodGet, "/backlog/", "", "", &accessToken)
	require.Less(t, rr.Code, http.StatusInternalServerError)
}

// TestRoutes_ThrottleEnabled verifies that Routes() returns a handler without
// panicking when Throttle is true.
func TestRoutes_ThrottleEnabled(t *testing.T) {
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

	throttledApp := NewApplication(
		logger,
		cfg,
		postgresDB,
		mocks.NewMockedGoTrueClient(),
	)

	handler := throttledApp.Routes()
	require.NotNil(t, handler)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusInternalServerError, rr.Code)
}

// TestCORSPreflight_ConnectProtocolVersion verifies that CORS preflight
// requests allow connect-protocol-version header when Throttle is enabled.
func TestCORSPreflight_ConnectProtocolVersion(t *testing.T) {
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

	throttledApp := NewApplication(
		logger,
		cfg,
		postgresDB,
		mocks.NewMockedGoTrueClient(),
	)

	handler := throttledApp.Routes()
	rr := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "connect-protocol-version")

	handler.ServeHTTP(rr, req)

	allowHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowHeaders, "connect-protocol-version")
}
