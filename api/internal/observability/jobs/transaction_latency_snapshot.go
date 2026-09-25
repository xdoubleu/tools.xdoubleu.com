package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/sentryapi"
)

const transactionLatencySnapshotRunEvery = 24 * time.Hour

type transactionStatsLister interface {
	ListTransactionStats(ctx context.Context) ([]sentryapi.TransactionStat, error)
}

type transactionLatencyInserter interface {
	Insert(ctx context.Context, day time.Time, stats []sentryapi.TransactionStat) error
}

// TransactionLatencySnapshotJob snapshots today's per-transaction p95 and
// count from Sentry into global.transaction_latency_daily, for Trends.
type TransactionLatencySnapshotJob struct {
	sentry transactionStatsLister
	repo   transactionLatencyInserter
}

func NewTransactionLatencySnapshotJob(
	sentry transactionStatsLister, repo transactionLatencyInserter,
) *TransactionLatencySnapshotJob {
	return &TransactionLatencySnapshotJob{sentry: sentry, repo: repo}
}

func (j *TransactionLatencySnapshotJob) ID() string {
	return "transaction-latency-snapshot"
}

func (j *TransactionLatencySnapshotJob) RunEvery() time.Duration {
	return transactionLatencySnapshotRunEvery
}

// Run no-ops when Sentry isn't configured; other errors propagate so
// TrackedJob records them and alerts.
func (j *TransactionLatencySnapshotJob) Run(
	ctx context.Context,
	logger *slog.Logger,
) error {
	stats, err := j.sentry.ListTransactionStats(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		logger.WarnContext(ctx,
			"transaction latency snapshot got zero stats from Sentry")
	}

	return j.repo.Insert(ctx, time.Now(), stats)
}
