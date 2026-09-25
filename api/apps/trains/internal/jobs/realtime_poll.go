package jobs

import (
	"context"
	"log/slog"
	"time"
)

// realtimePoller is the slice of services.RealtimeService the job needs.
type realtimePoller interface {
	Poll(ctx context.Context) error
}

// RealtimePollJob polls the BMC GTFS-Realtime feeds every ~30s, replacing
// the in-memory realtime snapshot each run.
type RealtimePollJob struct {
	svc realtimePoller
}

func NewRealtimePollJob(svc realtimePoller) *RealtimePollJob {
	return &RealtimePollJob{svc: svc}
}

func (j *RealtimePollJob) ID() string { return "trains-realtime-poll" }

func (j *RealtimePollJob) RunEvery() time.Duration {
	const pollInterval = 30 * time.Second
	return pollInterval
}

// pollTimeout keeps a slow poll (slow SNCB response, or a read blocked behind
// the static import) from running past the next 30s tick.
const pollTimeout = 20 * time.Second

func (j *RealtimePollJob) Run(ctx context.Context, _ *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()
	return j.svc.Poll(ctx)
}
