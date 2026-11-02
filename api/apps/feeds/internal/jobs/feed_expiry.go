package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/services"
)

// FeedExpiryJob hides unread items that outlived their feed's lifetime; it
// runs at startup too, so a lifetime change applies within the hour.
type FeedExpiryJob struct {
	feeds *services.FeedService
}

func NewFeedExpiryJob(feeds *services.FeedService) *FeedExpiryJob {
	return &FeedExpiryJob{feeds: feeds}
}

func (j *FeedExpiryJob) ID() string {
	return "expire-feed-items"
}

func (j *FeedExpiryJob) RunEvery() time.Duration {
	return time.Hour
}

func (j *FeedExpiryJob) Run(ctx context.Context, logger *slog.Logger) error {
	expired, err := j.feeds.ExpireDue(ctx)
	if err != nil {
		return err
	}
	if expired > 0 {
		logger.InfoContext(ctx, "expired feed items", "count", expired)
	}
	return nil
}
