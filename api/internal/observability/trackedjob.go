// Package observability records job runs in global.job_runs and request usage.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"tools.xdoubleu.com/internal/database/postgres"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/threading"
)

// jobDuration backs Grafana's JobP95High alert.
//
//nolint:gochecknoglobals //Prometheus collectors are process-wide by design
var jobDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "job_duration_seconds",
		Help:    "Duration of background job runs.",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
	},
	[]string{"job", "success"},
)

type jobRunsInserter interface {
	Insert(ctx context.Context, run models.JobRun) error
}

// TrackedJob wraps a threading.Job so every run is recorded in
// global.job_runs, failures log at Error (Sentry), and panics become failed
// runs. The inner job's ID is preserved.
type TrackedJob struct {
	inner threading.Job
	repo  jobRunsInserter
}

var _ threading.Job = (*TrackedJob)(nil)

// NewTrackedJob decorates inner, preserving threading.Scheduled if present.
func NewTrackedJob(inner threading.Job, db postgres.DB) threading.Job {
	tj := &TrackedJob{
		inner: inner,
		repo:  repositories.NewJobRunsRepository(db),
	}
	scheduled, ok := inner.(threading.Scheduled)
	if !ok {
		return tj
	}
	return &scheduledTrackedJob{TrackedJob: tj, scheduled: scheduled}
}

func (j *TrackedJob) ID() string {
	return j.inner.ID()
}

func (j *TrackedJob) Run(ctx context.Context, logger *slog.Logger) (err error) {
	start := time.Now()

	// One transaction per run: a per-worker one would never finish.
	transaction := sentry.StartTransaction(
		ctx,
		j.inner.ID(),
		sentry.WithOpName("job.run"),
	)
	transaction.Status = sentry.HTTPtoSpanStatus(http.StatusOK)
	ctx = transaction.Context()
	defer transaction.Finish()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
		}
		if err != nil {
			transaction.Status = sentry.HTTPtoSpanStatus(http.StatusInternalServerError)
		}
		j.record(ctx, logger, start, err)
	}()

	return j.inner.Run(ctx, logger)
}

func (j *TrackedJob) record(
	ctx context.Context,
	logger *slog.Logger,
	start time.Time,
	err error,
) {
	elapsed := time.Since(start)

	jobDuration.WithLabelValues(
		j.inner.ID(),
		strconv.FormatBool(err == nil),
	).Observe(elapsed.Seconds())

	run := models.JobRun{
		JobID:      j.inner.ID(),
		StartedAt:  start,
		DurationMs: elapsed.Milliseconds(),
		Success:    err == nil,
		Error:      "",
	}

	if err != nil {
		run.Error = err.Error()
		logger.ErrorContext(
			ctx,
			"job failed",
			slog.String("job", run.JobID),
			essentialogger.ErrAttr(err),
		)
	}

	// Best-effort: a failed insert must never fail the job.
	if insertErr := j.repo.Insert(ctx, run); insertErr != nil {
		logger.ErrorContext(
			ctx,
			"failed to record job run",
			slog.String("job", run.JobID),
			essentialogger.ErrAttr(insertErr),
		)
	}
}

// scheduledTrackedJob keeps a wrapped Scheduled job visible to JobQueue.
type scheduledTrackedJob struct {
	*TrackedJob
	scheduled threading.Scheduled
}

var _ threading.Scheduled = (*scheduledTrackedJob)(nil)

func (j *scheduledTrackedJob) RunEvery() time.Duration {
	return j.scheduled.RunEvery()
}
