package jobs

import (
	"context"

	"tools.xdoubleu.com/internal/models"
)

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
// above its class's threshold (classifyTransaction/thresholdMsForClass,
// issue #1310) — a transaction that merely got relatively slower without
// crossing its class's absolute threshold isn't "slow" for notification
// purposes, just for the trending list on /monitoring/observability.
// Excluded transactions (slowTransactionExcluded) never count as slow here
// either, for the same reason ThresholdAlertJob excludes them.
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
		if slowTransactionExcluded(t.Transaction) {
			continue
		}
		threshold := thresholdMsForClass(classifyTransaction(t.Transaction))
		if t.RecentAvgP95Ms >= threshold {
			slow = append(slow, t)
		}
	}
	return slow, nil
}
