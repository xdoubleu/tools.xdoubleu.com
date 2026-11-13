package observability

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsPath is scraped by Prometheus over the Docker network, so it's
// registered unprefixed like /health.
const MetricsPath = "/metrics"

// MetricsHandler exposes promhttp's default registry (go_*, process_*, and the
// browser's web_vitals_seconds). Host-level metrics come from node_exporter.
// The api is also the root service behind kamal-proxy, so /metrics is
// reachable publicly: a non-empty token is required as a bearer credential,
// which Prometheus sends (infra/prometheus.yml).
func MetricsHandler(token string) http.Handler {
	scrape := promhttp.Handler()
	if token == "" {
		return scrape
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		scrape.ServeHTTP(w, r)
	})
}
