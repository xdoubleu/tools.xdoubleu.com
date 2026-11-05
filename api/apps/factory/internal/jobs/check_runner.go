package jobs

import (
	"context"
	"log/slog"
	"time"
)

// runnerWatcher is the slice of services.JobService the job needs.
type runnerWatcher interface {
	CheckRunnerOffline(ctx context.Context) error
}

// CheckRunnerJob tells Slack when a job has waited for a runner that isn't there.
type CheckRunnerJob struct {
	jobs runnerWatcher
}

func NewCheckRunnerJob(jobs runnerWatcher) CheckRunnerJob {
	return CheckRunnerJob{jobs: jobs}
}

func (j CheckRunnerJob) ID() string {
	return "factory-check-runner"
}

func (j CheckRunnerJob) RunEvery() time.Duration {
	const minutes = 5
	return minutes * time.Minute
}

func (j CheckRunnerJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.jobs.CheckRunnerOffline(
		ctx,
	) //nolint:wrapcheck // the job runner names the job
}
