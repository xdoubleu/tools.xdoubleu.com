// Package observability records job runs in global.job_runs and request usage.
package observability

import (
	"context"
	"errors"
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

// DefaultRunDeadline bounds a run, well above every job's longest observed
// run; a job implementing Deadlined sets its own.
const DefaultRunDeadline = 30 * time.Minute

// Deadlined overrides DefaultRunDeadline for a job.
type Deadlined interface {
	RunDeadline() time.Duration
}

var errRunDeadline = errors.New("job exceeded its run deadline")

type jobRunsInserter interface {
	Insert(ctx context.Context, run models.JobRun) error
}

// TrackedJob wraps a threading.Job so every run is recorded in
// global.job_runs, failures log at Error (Sentry) or at Warn once ctx is
// cancelled, and panics or overrunning the deadline become failed runs. The
// inner job's ID is preserved.
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

	deadline := DefaultRunDeadline
	if d, ok := j.inner.(Deadlined); ok {
		deadline = d.RunDeadline()
	}
	runCtx, cancel := context.WithTimeoutCause(ctx, deadline, errRunDeadline)

	defer func() {
		cancel()
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
		}
		// After cancel the cause is final; the job may have swallowed it.
		if errors.Is(context.Cause(runCtx), errRunDeadline) {
			err = errors.Join(fmt.Errorf("%w of %s", errRunDeadline, deadline), err)
		}
		if err != nil {
			transaction.Status = sentry.HTTPtoSpanStatus(http.StatusInternalServerError)
		}
		j.record(ctx, logger, start, err)
	}()

	return j.inner.Run(runCtx, logger)
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

	// A cancelled ctx (shutdown, or a caller giving up) is not worth an alert.
	level := slog.LevelError
	if ctx.Err() != nil {
		level = slog.LevelWarn
	}
	if err != nil {
		run.Error = err.Error()
		logger.Log(
			ctx,
			level,
			"job failed",
			slog.String("job", run.JobID),
			essentialogger.ErrAttr(err),
		)
	}

	// Best-effort: a failed insert must never fail the job. Detached so a run
	// interrupted by shutdown is still recorded.
	if insertErr := j.repo.Insert(context.WithoutCancel(ctx), run); insertErr != nil {
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
