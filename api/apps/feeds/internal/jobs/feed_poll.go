package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/services"
)

// FeedPollJob polls every pollable feed; it runs at startup too, and
// conditional GETs keep quiet polls cheap.
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
