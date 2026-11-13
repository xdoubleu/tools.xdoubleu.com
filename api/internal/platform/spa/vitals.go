package spa

import (
	"encoding/json"
	"math"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// webVitals backs Grafana's FrontendP95High alert. Browser samples arrive in
// milliseconds (CLS is unitless); the histogram stores seconds with buckets
// that mirror http_request_duration_seconds.
//
//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var webVitals = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "web_vitals_seconds",
		Help: "Core Web Vitals reported by the browser, in seconds.",
		Buckets: []float64{
			.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10,
		},
	},
	[]string{"metric"},
)

// knownWebVitals bounds label cardinality; other names are dropped.
//
//nolint:gochecknoglobals //read-only allowlist
var knownWebVitals = map[string]bool{
	"CLS": true, "FCP": true, "INP": true, "LCP": true, "TTFB": true, "FID": true,
}

// recordWebVital observes one sample; it reports whether it was accepted.
func recordWebVital(name string, valueMs float64) bool {
	if !knownWebVitals[name] || math.IsNaN(valueMs) || math.IsInf(valueMs, 0) ||
		valueMs < 0 {
		return false
	}
	const msPerSecond = 1000
	webVitals.WithLabelValues(name).Observe(valueMs / msPerSecond)
	return true
}

// serveVitals is the unauthenticated same-origin Web Vitals beacon.
func (h *Handler) serveVitals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.admit(w, r, h.vitals, maxVitalsBodyBytes) {
		return
	}

	var body struct {
		Name  *string  `json:"name"`
		Value *float64 `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.Name == nil || body.Value == nil {
		http.Error(w, "expected {name: string, value: number}", http.StatusBadRequest)
		return
	}

	if recordWebVital(*body.Name, *body.Value) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
