package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsPath is scraped directly by Prometheus over the internal Docker
// network (infra/prometheus-compose.yml's api scrape job, issue #1468) —
// same pattern as healthPath: it must never go through kamal-proxy's public
// /api path_prefix (config/deploy.api.yml), so it's registered here
// unprefixed, exactly like /health, rather than under the ConnectRPC mux
// stripAPIPathPrefix (kamal_proxy_shim.go) rewrites.
const metricsPath = "/metrics"

// metricsHandler exposes the process/Go runtime metrics (go_*, process_*)
// promhttp's default registry already tracks, in Prometheus text-exposition
// format — the same handler client_golang's own examples use. This
// replaces the hand-rolled node_exporter-text-format parsing
// internal/observability/hostmetrics.go used to do for the api process
// itself; host-level CPU/memory/disk still come from node_exporter, scraped
// by Prometheus directly rather than by api.
func metricsHandler() http.Handler {
	return promhttp.Handler()
}
