package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// jobPhaseDuration times named phases of a job run. Prometheus relabels `job`
// to `job_name` (infra/prometheus.yml), so query
// `job_phase_duration_seconds{job_name=...}`.
//
//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var jobPhaseDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "job_phase_duration_seconds",
		Help: "Duration of a named phase within a background job run.",
		Buckets: []float64{
			0.1, 0.5, 1, 5, 10, 30, 60, 120, 300,
		},
	},
	[]string{"job", "phase"},
)

// ObserveJobPhase records one named phase's duration. Phases need not sum to
// the whole run.
func ObserveJobPhase(job, phase string, d time.Duration) {
	jobPhaseDuration.WithLabelValues(job, phase).Observe(d.Seconds())
}
