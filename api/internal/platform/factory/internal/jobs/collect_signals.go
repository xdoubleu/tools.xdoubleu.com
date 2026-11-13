package jobs

import (
	"context"
	"log/slog"
	"time"
)

// signalCollector is the slice of services.JobService the job needs.
type signalCollector interface {
	CollectSignals(ctx context.Context) error
}

// CollectSignalsJob keeps the factory failure gauges current.
type CollectSignalsJob struct {
	jobs signalCollector
}

func NewCollectSignalsJob(jobs signalCollector) CollectSignalsJob {
	return CollectSignalsJob{jobs: jobs}
}

func (j CollectSignalsJob) ID() string {
	return "factory-collect-signals"
}

func (j CollectSignalsJob) RunEvery() time.Duration {
	const minutes = 5
	return minutes * time.Minute
}

func (j CollectSignalsJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.jobs.CollectSignals(ctx) //nolint:wrapcheck // the job runner names the job
}
