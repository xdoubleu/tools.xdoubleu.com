package jobs

import (
	"context"
	"log/slog"
	"time"
)

// boardReconciler is the slice of services.JobService the job needs.
type boardReconciler interface {
	ReconcileBoard(ctx context.Context) (int, error)
}

// ReconcileBoardJob moves tasks the board shows in the wrong column: In progress
// without a job, or in Needs you after their pull request merged.
type ReconcileBoardJob struct {
	jobs boardReconciler
}

func NewReconcileBoardJob(jobs boardReconciler) ReconcileBoardJob {
	return ReconcileBoardJob{jobs: jobs}
}

func (j ReconcileBoardJob) ID() string {
	return "factory-reconcile-board"
}

func (j ReconcileBoardJob) RunEvery() time.Duration {
	const minutes = 5
	return minutes * time.Minute
}

func (j ReconcileBoardJob) Run(ctx context.Context, logger *slog.Logger) error {
	n, err := j.jobs.ReconcileBoard(ctx)
	if n > 0 {
		logger.InfoContext(ctx, "reconciled tasks", "tasks", n)
	}
	return err //nolint:wrapcheck // the job runner names the job
}
