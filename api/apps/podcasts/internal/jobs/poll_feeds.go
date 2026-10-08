package jobs

import (
	"context"
	"log/slog"
	"time"
)

// feedPoller is the slice of services.ShowService the job needs.
type feedPoller interface {
	PollAll(ctx context.Context, logger *slog.Logger) error
}

// PollFeedsJob fetches every favourite show's feed for new episodes.
type PollFeedsJob struct {
	shows feedPoller
}

func NewPollFeedsJob(shows feedPoller) PollFeedsJob {
	return PollFeedsJob{shows: shows}
}

func (j PollFeedsJob) ID() string {
	return "podcasts-poll-feeds"
}

func (j PollFeedsJob) RunEvery() time.Duration {
	return time.Hour
}

func (j PollFeedsJob) Run(ctx context.Context, logger *slog.Logger) error {
	return j.shows.PollAll(ctx, logger)
}
