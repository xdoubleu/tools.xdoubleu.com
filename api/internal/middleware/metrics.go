package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/communication/httptools"
)

// requestDuration backs Grafana's RequestP95High alert. The route label is
// r.Pattern (one series per route), with "other" for unmatched requests.
//
//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var requestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Duration of HTTP requests handled by the api server.",
		Buckets: []float64{
			.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10,
		},
	},
	[]string{"route", "method", "status"},
)

// unmeteredPaths are polled on a fixed cadence and would only add p95 noise.
//
//nolint:gochecknoglobals //effectively const
var unmeteredPaths = map[string]bool{
	"/metrics":     true,
	"/health":      true,
	"/api/version": true,
}

// RequestDuration records each request's duration in
// http_request_duration_seconds.
func RequestDuration() func(http.Handler) http.Handler {
	return requestDurationHandler
}

func requestDurationHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unmeteredPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		rw := httptools.NewResponseWriter(w)
		start := time.Now()

		next.ServeHTTP(rw, r)

		route := r.Pattern
		if route == "" {
			route = "other"
		}

		requestDuration.WithLabelValues(
			route,
			r.Method,
			statusClass(rw.Status()),
		).Observe(time.Since(start).Seconds())
	})
}

const statusClassDivisor = 100

func statusClass(status int) string {
	if status == 0 {
		status = http.StatusOK
	}
	return fmt.Sprintf("%dxx", status/statusClassDivisor)
}
