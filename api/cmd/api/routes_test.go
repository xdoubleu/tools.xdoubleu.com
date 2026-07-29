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

// TestAppAccess_DeniedReturns403 covers the AppAccess "denied" branch: every
// AppAccess call site guards a ConnectRPC POST handler (see apps/*/routes.go),
// never an HTML page, so a denial must respond with a plain 403 rather than
// AdminAccess's redirect — a fetch()-based RPC client follows a 30x
// transparently and would otherwise get back the home page's HTML instead of
// a usable error (issue #673).
func TestAppAccess_DeniedReturns403(t *testing.T) {
	demoteToUser(t)
	revokeAppAccess(t, testUserID, "todos")

	rr := doInProcess(
		t,
		http.MethodPost,
		"/todos.v1.TaskService/ListTasks",
		"{}",
		"application/json",
		&accessToken,
	)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
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

// deadlineSettingRecorder wraps httptest.ResponseRecorder to additionally
// implement http.ResponseController's SetWriteDeadline, which
// ResponseRecorder alone does not — withExtendedDeadline falls back to a
// no-op when the underlying writer doesn't support it, so exercising the
// "deadline actually applied" branch needs a writer that does.
type deadlineSettingRecorder struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

//nolint:unparam // satisfies http.ResponseController's interface signature
func (r *deadlineSettingRecorder) SetWriteDeadline(t time.Time) error {
	r.deadline = t
	return nil
}

// TestWithExtendedDeadline_AppliesWriteAndContextDeadline verifies that, when
// the ResponseWriter supports it, withExtendedDeadline both sets the
// response write deadline and gives the downstream handler's request context
// a matching deadline — the mechanism GetDeployLogs relies on to survive past
// the server's global 10s httpWriteTimeout (see routes.go).
func TestWithExtendedDeadline_AppliesWriteAndContextDeadline(t *testing.T) {
	//nolint:exhaustruct // deadline is set by the handler under test, not the fixture
	rec := &deadlineSettingRecorder{ResponseRecorder: httptest.NewRecorder()}

	var gotDeadline time.Time
	var gotOK bool
	handler := withExtendedDeadline(
		30*time.Second,
		func(_ http.ResponseWriter, r *http.Request) {
			gotDeadline, gotOK = r.Context().Deadline()
		},
	)

	before := time.Now()
	handler(rec, httptest.NewRequest(http.MethodPost, "/x", nil))

	require.True(t, gotOK, "downstream handler must see a context deadline")
	assert.WithinDuration(t, before.Add(30*time.Second), gotDeadline, time.Second)
	assert.WithinDuration(t, before.Add(30*time.Second), rec.deadline, time.Second)
}

// TestWithExtendedDeadline_FallsBackWhenUnsupported verifies that a
// ResponseWriter without SetWriteDeadline support (e.g. a plain
// httptest.ResponseRecorder) still reaches the downstream handler instead of
// erroring out.
func TestWithExtendedDeadline_FallsBackWhenUnsupported(t *testing.T) {
	called := false
	handler := withExtendedDeadline(
		30*time.Second,
		func(_ http.ResponseWriter, _ *http.Request) { called = true },
	)

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", nil))

	assert.True(t, called)
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
