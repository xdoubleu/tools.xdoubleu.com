// Package observability provides shared instrumentation: a job wrapper that
// records every run in global.job_runs and a request-usage recorder backing
// the admin dashboard.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

// jobRunsInserter is the slice of JobRunsRepository TrackedJob needs.
type jobRunsInserter interface {
	Insert(ctx context.Context, run models.JobRun) error
}

// TrackedJob decorates a threading.Job so every run — including ones the
// JobQueue would silently discard the error of — is recorded in
// global.job_runs and failures are logged at Error level (which the Sentry
// log handler forwards). Panics are captured as failed runs instead of
// killing the worker. The inner job's ID is preserved, so progress
// WebSocket topics and ForceRun keep working.
type TrackedJob struct {
	inner threading.Job
	repo  jobRunsInserter
}

var _ threading.Job = (*TrackedJob)(nil)

func NewTrackedJob(inner threading.Job, db postgres.DB) *TrackedJob {
	return &TrackedJob{
		inner: inner,
		repo:  repositories.NewJobRunsRepository(db),
	}
}

func (j *TrackedJob) ID() string {
	return j.inner.ID()
}

func (j *TrackedJob) RunEvery() time.Duration {
	return j.inner.RunEvery()
}

func (j *TrackedJob) Run(ctx context.Context, logger *slog.Logger) (err error) {
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
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
	run := models.JobRun{
		JobID:      j.inner.ID(),
		StartedAt:  start,
		DurationMs: time.Since(start).Milliseconds(),
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

	// Recording is best-effort: a failed insert must never fail the job.
	if insertErr := j.repo.Insert(ctx, run); insertErr != nil {
		logger.ErrorContext(
			ctx,
			"failed to record job run",
			slog.String("job", run.JobID),
			essentialogger.ErrAttr(insertErr),
		)
	}
}
