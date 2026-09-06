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

// RealtimePollJob polls the BMC gateway's GTFS-Realtime feeds on the
// gateway's recommended ~30s cadence (issue #1389/#1393), replacing the
// in-memory realtime snapshot every run.
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

func (j *RealtimePollJob) Run(ctx context.Context, _ *slog.Logger) error {
	return j.svc.Poll(ctx)
}
