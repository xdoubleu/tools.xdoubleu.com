package jobs

import (
	"context"
	"log/slog"
	"time"

	essentialogger "tools.xdoubleu.com/internal/logging"
)

// staleActionMaxAge is how long a global.automated_actions row may stay open
// before the sweep closes it as failed. The longest observed successful
// scheduled-routine run (ready-issues-executor) closes out after ~5h, so 24h
// leaves a generous margin before a genuinely-stalled row is force-closed —
// while still bounding how long the AutomatedActionStalled alert can fire on
// a routine that never reached its own close call (issue #1796).
const staleActionMaxAge = 24 * time.Hour

// sweepRunEvery is this job's own poll interval — slower than the package's
// shared runEvery (5m) since closing a stale row is not time-critical: the
// alert has already been firing for hours by the time the cutoff is crossed.
const sweepRunEvery = 15 * time.Minute

// automatedActionStaleCloser is the subset of
// *repositories.AutomatedActionsRepository the sweep needs.
type automatedActionStaleCloser interface {
	CloseStale(ctx context.Context, cutoff time.Time) ([]int64, error)
}

// AutomatedActionSweepJob periodically closes global.automated_actions rows
// that have been open longer than staleActionMaxAge, recording them as
// outcome=failed. Scheduled routines run outside api's own process and close
// their own rows via record_action(mode=close); a routine that crashes,
// times out, or loses its MCP connector before that step leaves its row open
// indefinitely, which would otherwise keep the AutomatedActionStalled alert
// firing until someone manually closes the row.
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
