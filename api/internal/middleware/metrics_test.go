package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/middleware"
)

func TestRequestDurationRecordsHistogram(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /widgets/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	handler := middleware.RequestDuration()(mux)
	before := requestCount(t, "GET /widgets/{id}", "4xx")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/widgets/42", nil))
	assert.Equal(t, http.StatusTeapot, rec.Code)

	assert.Equal(t, before+1, requestCount(t, "GET /widgets/{id}", "4xx"))
}

// A handler that sets no status gets net/http's implicit 200; it must be
// recorded as 2xx, not "0xx". The handlers are the shapes that skip
// WriteHeader: a plain body write (the MCP SDK, webpub JSON), io.Copy of a
// reader without WriterTo (webpub resources), and no write at all.
func TestRequestDurationRecordsImplicitOKAs2xx(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /implicit-write", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /implicit-copy", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.Copy(w, struct{ io.Reader }{strings.NewReader("ok")})
	})
	mux.HandleFunc("GET /implicit-empty", func(http.ResponseWriter, *http.Request) {})

	handler := middleware.RequestDuration()(mux)
	routes := []string{
		"GET /implicit-write", "GET /implicit-copy", "GET /implicit-empty",
	}
	before := make(map[string]int, len(routes))
	for _, route := range routes {
		before[route] = requestCount(t, route, "2xx")
	}

	for _, route := range routes {
		_, path, _ := strings.Cut(route, " ")
		handler.ServeHTTP(
			httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil),
		)
	}

	for _, route := range routes {
		assert.Equal(t, before[route]+1, requestCount(t, route, "2xx"), route)
		assert.Zero(t, requestCount(t, route, "0xx"), route)
	}
}

func TestRequestDurationSkipsInfraPaths(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.RequestDuration()(mux)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	assert.NotContains(t, scrapeMetrics(t), `route="/health"`)
}

func scrapeMetrics(t *testing.T) string {
	t.Helper()

	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(
		rec, httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

// A client-chosen method token doesn't mint a new series.
func TestRequestDurationBucketsUnknownMethods(t *testing.T) {
	t.Parallel()

	handler := middleware.RequestDuration()(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) },
	))
	handler.ServeHTTP(
		httptest.NewRecorder(), httptest.NewRequest("BREW", "/unknown-method", nil),
	)

	metrics := scrapeMetrics(t)
	assert.NotContains(t, metrics, `method="BREW"`)
	assert.Contains(t, metrics, `method="other"`)
}
