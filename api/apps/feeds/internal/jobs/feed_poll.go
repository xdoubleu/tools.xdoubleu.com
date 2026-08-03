package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/services"
)

// FeedPollJob periodically polls every RSS/Atom subscription and ingests new
// items. Unlike jobs that need arming, the startup run is desirable, and
// conditional GETs make quiet polls nearly free.
type FeedPollJob struct {
	feeds *services.FeedService
}

func NewFeedPollJob(feeds *services.FeedService) *FeedPollJob {
	return &FeedPollJob{feeds: feeds}
}

func (j *FeedPollJob) ID() string {
	return "poll-feeds"
}

func (j *FeedPollJob) RunEvery() time.Duration {
	return time.Hour
}

func (j *FeedPollJob) Run(ctx context.Context, logger *slog.Logger) error {
	return j.feeds.PollAll(ctx, logger, nil)
}
