// Package jobs holds the factory's recurring background jobs.
package jobs

import (
	"context"
	"log/slog"
	"time"
)

// eventPruner is the slice of services.JobService the job needs.
type eventPruner interface {
	PruneEvents(ctx context.Context) (int64, error)
}

// PruneJobEventsJob deletes job logs past their retention.
type PruneJobEventsJob struct {
	jobs eventPruner
}

func NewPruneJobEventsJob(jobs eventPruner) PruneJobEventsJob {
	return PruneJobEventsJob{jobs: jobs}
}

func (j PruneJobEventsJob) ID() string {
	return "factory-prune-job-events"
}

func (j PruneJobEventsJob) RunEvery() time.Duration {
	const hoursInDay = 24
	return hoursInDay * time.Hour
}

func (j PruneJobEventsJob) Run(ctx context.Context, logger *slog.Logger) error {
	n, err := j.jobs.PruneEvents(ctx)
	if err != nil {
		return err //nolint:wrapcheck // the job runner names the job
	}
	logger.InfoContext(ctx, "pruned job events", "deleted", n)
	return nil
}
