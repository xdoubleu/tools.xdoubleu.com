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

// pollTimeout bounds one poll cycle so a slow step — a slow SNCB response,
// or a trains.trips read blocked behind trains-static-import's TRUNCATE
// lock (issue #1720) — can't run past the next scheduled tick. It leaves
// the same margin against RunEvery's 30s cadence that ADR-0017's
// deployLogsCtxTimeout leaves against its own proxy ceiling.
const pollTimeout = 20 * time.Second

func (j *RealtimePollJob) Run(ctx context.Context, _ *slog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()
	return j.svc.Poll(ctx)
}
