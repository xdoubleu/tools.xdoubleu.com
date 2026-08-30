package jobs

import (
	"context"

	"tools.xdoubleu.com/internal/models"
)

// slowTransactionP95ThresholdMs is a provisional, single global "slow"
// cutoff shared by IssueNotifierJob and WeeklyDigestJob. Issue #1310 will
// replace this with per-class thresholds (HTTP handler / page load /
// background job) evaluated by ThresholdAlertJob; until then this is the
// single conservative constant #1310 itself calls out as an acceptable
// fallback — 5s is well above a typical API/page p95 (0.9-2s) but comfortably
// under the 20s deployLogsCtxTimeout ceiling documented in root CLAUDE.md.
const slowTransactionP95ThresholdMs = 5000.0

// pctChangeToPercent converts TransactionTrend.PctChange (a 0.20 = +20%
// fraction) to a whole-number percent for display.
const pctChangeToPercent = 100

// slowTransactionsRepo is the subset of
// *repositories.TransactionLatencyRepository this package needs.
type slowTransactionsRepo interface {
	Trends(ctx context.Context) ([]models.TransactionTrend, error)
}

// currentlySlowTransactions returns the regressing transactions (already
// filtered to a >=20% p95 increase by Trends) whose recent p95 is also at or
// above slowTransactionP95ThresholdMs — a transaction that merely got
// relatively slower without crossing the absolute threshold isn't "slow"
// for notification purposes, just for the trending list on
// /monitoring/observability.
func currentlySlowTransactions(
	ctx context.Context,
	repo slowTransactionsRepo,
) ([]models.TransactionTrend, error) {
	trends, err := repo.Trends(ctx)
	if err != nil {
		return nil, err
	}

	slow := make([]models.TransactionTrend, 0, len(trends))
	for _, t := range trends {
		if t.RecentAvgP95Ms >= slowTransactionP95ThresholdMs {
			slow = append(slow, t)
		}
	}
	return slow, nil
}
