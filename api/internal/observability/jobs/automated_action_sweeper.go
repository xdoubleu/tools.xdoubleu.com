package jobs

import (
	"context"
	"log/slog"
	"time"

	essentialogger "tools.xdoubleu.com/internal/logging"
)

// staleActionMaxAge is how long an automated_actions row may stay open before
// the sweep fails it; the longest successful routine run takes ~5h.
const staleActionMaxAge = 24 * time.Hour

// sweepRunEvery is slower than runEvery; closing stale rows isn't urgent.
const sweepRunEvery = 15 * time.Minute

type automatedActionStaleCloser interface {
	CloseStale(ctx context.Context, cutoff time.Time) ([]int64, error)
}

// AutomatedActionSweepJob closes rows open longer than staleActionMaxAge as
// failed. Routines close their own rows, but a crashed one never does,
// leaving AutomatedActionStalled firing.
type AutomatedActionSweepJob struct {
	automatedActions automatedActionStaleCloser
}

func NewAutomatedActionSweepJob(
	automatedActions automatedActionStaleCloser,
) *AutomatedActionSweepJob {
	return &AutomatedActionSweepJob{automatedActions: automatedActions}
}

func (j *AutomatedActionSweepJob) ID() string {
	return "sweep-automated-actions"
}

func (j *AutomatedActionSweepJob) RunEvery() time.Duration {
	return sweepRunEvery
}

func (j *AutomatedActionSweepJob) Run(
	ctx context.Context,
	logger *slog.Logger,
) error {
	ids, err := j.automatedActions.CloseStale(
		ctx, time.Now().Add(-staleActionMaxAge),
	)
	if err != nil {
		logger.ErrorContext(ctx,
			"automated-action-sweep: failed to close stale rows",
			essentialogger.ErrAttr(err))
		return nil
	}
	if len(ids) > 0 {
		logger.InfoContext(ctx,
			"automated-action-sweep: closed stale automated_actions rows",
			slog.Int("count", len(ids)),
			slog.Any("ids", ids),
			slog.Duration("max_age", staleActionMaxAge))
	}
	return nil
}
