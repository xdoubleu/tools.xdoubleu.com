package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/communication/httptools"
)

// requestDuration is the Prometheus histogram Grafana's RequestP95High alert
// rule evaluates (issue #1528). It's registered on client_golang's default
// registry, which cmd/api's /metrics handler (promhttp.Handler) already
// serves, so no registry plumbing is needed.
//
// The route label is the stdlib mux pattern (r.Pattern), which is inherently
// low-cardinality — one series per registered route, never per request path —
// with a single "other" bucket for the handful of requests that match no
// pattern.
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

// unmeteredPaths are infrastructure endpoints kept out of the latency
// histogram: they're scraped/polled on a fixed cadence and would only add
// noise to the p95.
//
//nolint:gochecknoglobals //effectively const
var unmeteredPaths = map[string]bool{
	"/metrics":     true,
	"/health":      true,
	"/api/version": true,
}

// RequestDuration is middleware that records every request's wall-clock
// duration into the http_request_duration_seconds histogram.
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

// statusClassDivisor collapses a 3-digit status code to its leading digit.
const statusClassDivisor = 100

// statusClass buckets a status code as "2xx"/"4xx"/"5xx" etc. so the label
// stays low-cardinality.
func statusClass(status int) string {
	if status == 0 {
		status = http.StatusOK
	}
	return fmt.Sprintf("%dxx", status/statusClassDivisor)
}
