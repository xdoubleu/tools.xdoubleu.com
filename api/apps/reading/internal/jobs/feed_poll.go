package jobs

import (
	"context"
	"log/slog"
	"time"

	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/internal/progressws"
)

// FeedPollJob periodically polls every RSS/Atom subscription and ingests new
// items. Unlike ResyncMetadataJob it needs no arming — the startup run is
// desirable, and conditional GETs make quiet polls nearly free.
type FeedPollJob struct {
	feeds *services.FeedService
	ws    *progressws.Service
}

func NewFeedPollJob(
	feeds *services.FeedService,
	ws *progressws.Service,
) *FeedPollJob {
	return &FeedPollJob{feeds: feeds, ws: ws}
}

func (j *FeedPollJob) ID() string {
	return "poll-feeds"
}

func (j *FeedPollJob) RunEvery() time.Duration {
	return time.Hour
}

func (j *FeedPollJob) Run(ctx context.Context, logger *slog.Logger) error {
	var onProgress func(int, int)
	if j.ws != nil {
		id := j.ID()
		onProgress = func(processed, total int) {
			j.ws.UpdateProgress(id, processed, total)
		}
	}
	return j.feeds.PollAll(ctx, logger, onProgress)
}
