package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/config"
)

// stubUpstreamPort starts an httptest.Server standing in for the Next.js
// child and returns the port frontendProxy should target.
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

func proxyTestApp(webPort int) *Application {
	//nolint:exhaustruct //only the fields frontendProxy reads are needed
	return &Application{
		logger: logging.NewNopLogger(),
		config: config.Config{WebPort: webPort},
	}
}

func TestFrontendProxy_RoutesToAPIHandler(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		wantAPIPath string
	}{
		{"health unstripped", "/health", "/health"},
		{"bare api prefix", "/api", "/"},
		{"api subpath stripped", "/api/foo/bar", "/foo/bar"},
		{
			"preserved double-api quirk",
			"/api/api/version",
			"/api/version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			apiHandler := http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					gotPath = r.URL.Path
					w.WriteHeader(http.StatusOK)
				},
			)
			// Upstream must exist but should never be hit for these cases.
			port := stubUpstreamPort(t, http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					t.Error("proxy target should not have been reached")
					w.WriteHeader(http.StatusOK)
				},
			))
			app := proxyTestApp(port)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			app.frontendProxy(apiHandler).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.wantAPIPath, gotPath)
		})
	}
}

func TestFrontendProxy_ProxiesEverythingElse(t *testing.T) {
	tests := []string{"/", "/reading/123", "/release", "/games/distribution/1"}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			apiHandler := http.HandlerFunc(
				func(_ http.ResponseWriter, _ *http.Request) {
					t.Error("api handler should not have been reached")
				},
			)
			port := stubUpstreamPort(t, http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("X-Proxied", "1")
					w.WriteHeader(http.StatusOK)
				},
			))
			app := proxyTestApp(port)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			app.frontendProxy(apiHandler).ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, "1", rr.Header().Get("X-Proxied"))
		})
	}
}

func TestFrontendProxy_DeadUpstreamReturns503(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	))
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	ts.Close() // stub is dead before any request reaches it, port refuses connections
	app := proxyTestApp(port)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	app.frontendProxy(http.NotFoundHandler()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// TestFrontendProxy_VersionQuirkAgainstRealMux exercises the preserved
// /api/api/version quirk end-to-end against the real api mux (GET
// /api/version is registered on it directly — see routes.go), not just the
// routing logic in isolation.
func TestFrontendProxy_VersionQuirkAgainstRealMux(t *testing.T) {
	port := stubUpstreamPort(t, http.HandlerFunc(
		func(_ http.ResponseWriter, _ *http.Request) {},
	))
	proxied := &Application{ //nolint:exhaustruct //other fields unused here
		logger: testApp.logger,
		config: config.Config{WebPort: port, Release: testApp.config.Release},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/api/version", nil)
	proxied.frontendProxy(testApp.Routes()).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "release")
}
