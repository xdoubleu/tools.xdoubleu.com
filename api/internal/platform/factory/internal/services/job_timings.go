package services

import (
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

const (
	// maxRetries bounds what a runner may claim, so a bad report can't skew counters.
	maxRetries = 100
	// maxSeconds is a week, far beyond any job.
	maxSeconds = 7 * 24 * 3600
)

//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var (
	jobPhaseSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "factory_job_phase_seconds",
		Help:    "Time a finished job spent per phase (agent, publish, ci, merge).",
		Buckets: []float64{10, 30, 60, 120, 300, 600, 1200, 1800, 3600, 7200},
	}, []string{"phase"})
	jobAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "factory_job_attempts_total",
		Help: "Repeated attempts of finished jobs, by the stage that failed.",
	}, []string{"cause"})
	jobTimeToMerge = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "factory_job_time_to_merge_seconds",
		Help:    "Time from a succeeded job's first prompt to its merge.",
		Buckets: []float64{60, 300, 600, 1200, 1800, 3600, 7200, 14400},
	})
)

// RecordTimings stores a finished job's phase times and exports them. Only the
// runner that ran the job may report for it.
func (s *JobService) RecordTimings(
	ctx context.Context,
	runnerID, jobID uuid.UUID,
	t models.JobTimings,
	succeeded bool,
) error {
	t.Agent, t.Publish = clampSeconds(t.Agent), clampSeconds(t.Publish)
	t.CI, t.Merge = clampSeconds(t.CI), clampSeconds(t.Merge)
	t.Total = clampSeconds(t.Total)
	t.GateRetries = clampRetries(t.GateRetries)
	t.PushRetries = clampRetries(t.PushRetries)
	t.CIRetries = clampRetries(t.CIRetries)
	if err := s.jobs.RecordTimings(ctx, jobID, runnerID, t); err != nil {
		return err
	}
	jobPhaseSeconds.WithLabelValues("agent").Observe(t.Agent)
	jobPhaseSeconds.WithLabelValues("publish").Observe(t.Publish)
	jobPhaseSeconds.WithLabelValues("ci").Observe(t.CI)
	jobPhaseSeconds.WithLabelValues("merge").Observe(t.Merge)
	jobAttempts.WithLabelValues("gate").Add(float64(t.GateRetries))
	jobAttempts.WithLabelValues("push").Add(float64(t.PushRetries))
	jobAttempts.WithLabelValues("ci").Add(float64(t.CIRetries))
	if succeeded {
		jobTimeToMerge.Observe(t.Total)
	}
	return nil
}

func clampSeconds(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	return min(v, maxSeconds)
}

func clampRetries(n int) int { return min(max(n, 0), maxRetries) }
