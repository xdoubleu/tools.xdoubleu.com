package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsPath is scraped by Prometheus over the Docker network, bypassing
// kamal-proxy, so it's registered unprefixed like /health.
const metricsPath = "/metrics"

// metricsHandler exposes promhttp's default registry (go_*, process_*).
// Host-level metrics come from node_exporter.
func metricsHandler() http.Handler {
	return promhttp.Handler()
}
