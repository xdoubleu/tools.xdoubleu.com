package gateway_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/gateway"
	"tools.xdoubleu.com/gateway/internal/logging"
)

// brokenResponseWriter fails every Write, to exercise NewHandler's
// WriteJSON-error logging branch on the /gateway/version route.
type brokenResponseWriter struct {
	http.ResponseWriter
}

func (brokenResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// stubUpstreamPort starts an httptest.Server standing in for a child
// process and returns the port NewHandler should target.
func stubUpstreamPort(t *testing.T, handler http.Handler) int {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	return port
}

func TestNewHandler_RoutesToAPI(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		wantAPIPath string
	}{
		{"health unstripped", "/health", "/health"},
		{
			"well-known unstripped", "/.well-known/oauth-protected-resource",
			"/.well-known/oauth-protected-resource",
		},
		{"bare api prefix", "/api", "/"},
		{"api subpath stripped", "/api/foo/bar", "/foo/bar"},
		{"preserved double-api quirk", "/api/api/version", "/api/version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			apiPort := stubUpstreamPort(t, http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					gotPath = r.URL.Path
					w.WriteHeader(http.StatusOK)
				},
			))
			// Upstream must exist but should never be hit for these cases.
			webPort := stubUpstreamPort(t, http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					t.Error("web proxy should not have been reached")
					w.WriteHeader(http.StatusOK)
				},
			))
			handler := gateway.NewHandler(apiPort, webPort, "abc1234", logging.NewNopLogger())

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.wantAPIPath, gotPath)
		})
	}
}

func TestNewHandler_VersionAnsweredDirectly(t *testing.T) {
	apiPort := stubUpstreamPort(t, http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("api proxy should not have been reached")
		},
	))
	webPort := stubUpstreamPort(t, http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("web proxy should not have been reached")
		},
	))
	handler := gateway.NewHandler(apiPort, webPort, "abc1234", logging.NewNopLogger())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/gateway/version", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"release": "abc1234"}`, rr.Body.String())
}

func TestNewHandler_VersionLogsWriteFailure(t *testing.T) {
	apiPort := stubUpstreamPort(t, http.NotFoundHandler())
	webPort := stubUpstreamPort(t, http.NotFoundHandler())
	handler := gateway.NewHandler(apiPort, webPort, "abc1234", logging.NewNopLogger())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/gateway/version", nil)

	// Only assert this doesn't panic — the broken writer's error is logged,
	// not surfaced to the caller, matching NewHandler's other error paths.
	assert.NotPanics(t, func() {
		handler.ServeHTTP(brokenResponseWriter{ResponseWriter: rr}, req)
	})
}

func TestNewHandler_ProxiesEverythingElseToWeb(t *testing.T) {
	tests := []string{"/", "/books/123", "/release", "/games/distribution/1"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			apiPort := stubUpstreamPort(t, http.HandlerFunc(
				func(_ http.ResponseWriter, _ *http.Request) {
					t.Error("api proxy should not have been reached")
				},
			))
			webPort := stubUpstreamPort(t, http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("X-Proxied", "1")
					w.WriteHeader(http.StatusOK)
				},
			))
			handler := gateway.NewHandler(apiPort, webPort, "abc1234", logging.NewNopLogger())

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "1", rr.Header().Get("X-Proxied"))
		})
	}
}

func TestNewHandler_DeadAPIUpstreamReturns503(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	))
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	apiPort, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	ts.Close() // stub is dead before any request reaches it, port refuses connections

	webPort := stubUpstreamPort(t, http.NotFoundHandler())
	handler := gateway.NewHandler(apiPort, webPort, "abc1234", logging.NewNopLogger())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestNewHandler_DeadWebUpstreamReturns503(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	))
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	webPort, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	ts.Close()

	apiPort := stubUpstreamPort(t, http.NotFoundHandler())
	handler := gateway.NewHandler(apiPort, webPort, "abc1234", logging.NewNopLogger())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
