package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"tools.xdoubleu.com/internal/sentryapi"
)

// transactionLatencySnapshotRunEvery matches the daily granularity of
// global.transaction_latency_daily — one row per (day, project,
// transaction).
const transactionLatencySnapshotRunEvery = 24 * time.Hour

// transactionStatsLister is the slice of sentryapi.Client
// TransactionLatencySnapshotJob needs.
type transactionStatsLister interface {
	ListTransactionStats(ctx context.Context) ([]sentryapi.TransactionStat, error)
}

// transactionLatencyInserter is the slice of
// *repositories.TransactionLatencyRepository TransactionLatencySnapshotJob
// needs.
type transactionLatencyInserter interface {
	Insert(ctx context.Context, day time.Time, stats []sentryapi.TransactionStat) error
}

// TransactionLatencySnapshotJob snapshots today's per-transaction p95
// duration/request count from Sentry into global.transaction_latency_daily
// once a day (issue #848), building the history the "getting slower" trend
// comparison (TransactionLatencyRepository.Trends) needs. Cross-app, like
// IssueNotifierJob — this is observability data, not scoped to one
// apps/<name>.
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

// Run no-ops when Sentry isn't configured yet — matching the dashboard's own
// degrade-gracefully convention — instead of failing every run until an
// admin connects it. Any other error propagates so TrackedJob records the
// failure in global.job_runs and logs it at Error (reaching Sentry), which
// is the signal an admin needs for "the daily snapshot didn't happen".
func (j *TransactionLatencySnapshotJob) Run(ctx context.Context, _ *slog.Logger) error {
	stats, err := j.sentry.ListTransactionStats(ctx)
	if errors.Is(err, sentryapi.ErrNotConfigured) {
		return nil
	}
	if err != nil {
		return err
	}

	return j.repo.Insert(ctx, time.Now(), stats)
}
