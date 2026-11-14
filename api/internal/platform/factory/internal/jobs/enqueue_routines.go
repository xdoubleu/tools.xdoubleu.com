package jobs

import (
	"context"
	"log/slog"
	"time"
)

// routineEnqueuer is the slice of services.JobService the job needs.
type routineEnqueuer interface {
	EnqueueDueRoutines(ctx context.Context) error
}

// EnqueueRoutinesJob queues a task-less job for each routine that is due.
type EnqueueRoutinesJob struct {
	jobs routineEnqueuer
}

func NewEnqueueRoutinesJob(jobs routineEnqueuer) EnqueueRoutinesJob {
	return EnqueueRoutinesJob{jobs: jobs}
}

func (j EnqueueRoutinesJob) ID() string {
	return "factory-enqueue-routines"
}

func (j EnqueueRoutinesJob) RunEvery() time.Duration {
	return time.Minute
}

func (j EnqueueRoutinesJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.jobs.EnqueueDueRoutines(
		ctx,
	) //nolint:wrapcheck // the job runner names the job
}
