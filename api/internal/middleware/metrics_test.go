package middleware_test

import (
	"net/http"
	"net/http/httptest"
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

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/widgets/42", nil))
	assert.Equal(t, http.StatusTeapot, rec.Code)

	metrics := scrapeMetrics(t)
	assert.Regexp(
		t,
		`http_request_duration_seconds_count\{[^}]*route="GET /widgets/\{id\}"[^}]*\} 1`,
		metrics,
	)
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
