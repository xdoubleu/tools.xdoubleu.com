package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// jobPhaseDuration breaks a single job run down into named phases, so a slow
// run can be attributed to one phase (a fetch, a parse, a bulk import step)
// rather than only measured end to end. Registered on client_golang's default
// registry, which cmd/api's /metrics handler already serves.
//
// Like jobDuration's, this histogram's `job` label collides with the scrape
// config's `job` label; infra/prometheus.yml's metric_relabel_configs rename
// the exposed one to `job_name` for every metric of the api scrape, so this
// series is queryable as `job_phase_duration_seconds{job_name=...}`.
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

// ObserveJobPhase records how long one named phase of a job run took. Any
// job whose runtime splits into distinguishable steps should observe each
// one with a stable phase name; the phases of one run need not sum to the
// whole (a run may skip phases, e.g. a conditional-GET no-op).
func ObserveJobPhase(job, phase string, d time.Duration) {
	jobPhaseDuration.WithLabelValues(job, phase).Observe(d.Seconds())
}
