package jobs

import (
	"context"
	"log/slog"
	"time"
)

// pullRequestPoller is the slice of services.JobService the job needs.
type pullRequestPoller interface {
	PollPullRequests(ctx context.Context) (int, error)
}

// PollPullRequestsJob keeps each job's pull request status current and moves its
// task when the pull request ends. Polling needs no public webhook endpoint.
type PollPullRequestsJob struct {
	jobs pullRequestPoller
}

func NewPollPullRequestsJob(jobs pullRequestPoller) PollPullRequestsJob {
	return PollPullRequestsJob{jobs: jobs}
}

func (j PollPullRequestsJob) ID() string {
	return "factory-poll-pull-requests"
}

func (j PollPullRequestsJob) RunEvery() time.Duration {
	const minutes = 5
	return minutes * time.Minute
}

func (j PollPullRequestsJob) Run(ctx context.Context, logger *slog.Logger) error {
	n, err := j.jobs.PollPullRequests(ctx)
	if err != nil {
		return err //nolint:wrapcheck // the job runner names the job
	}
	if n > 0 {
		logger.InfoContext(ctx, "polled pull requests", "count", n)
	}
	return nil
}
