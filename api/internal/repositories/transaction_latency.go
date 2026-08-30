package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

const (
	// transactionLatencyRetention bounds
	// global.transaction_latency_daily; older rows are pruned on insert.
	// Shorter than storage_snapshots'/usage_daily's ~13 months — this data
	// only feeds a recent-vs-prior regression comparison, not a
	// year-over-year chart.
	transactionLatencyRetention = 90 * 24 * time.Hour

	// trendRecentWindow/trendPriorWindow are the two adjacent windows
	// Trends compares: the last 7 days against the 7 days before that.
	trendRecentWindow = 7 * 24 * time.Hour
	trendPriorWindow  = 7 * 24 * time.Hour
	// trendMinPriorP95Ms floors out near-zero baselines, where a tiny
	// absolute increase would otherwise register as a huge percentage
	// change.
	trendMinPriorP95Ms = 50.0
	// trendMinPctChange is the minimum p95 increase (recent vs prior) for a
	// transaction to be reported as regressing.
	trendMinPctChange = 0.20
	// trendLimit caps how many regressing transactions Trends returns.
	trendLimit = 20
)

type TransactionLatencyRepository struct {
	db postgres.DB
}

func NewTransactionLatencyRepository(db postgres.DB) *TransactionLatencyRepository {
	return &TransactionLatencyRepository{db: db}
}

// Insert upserts one day's transaction stats — each row is the latest
// snapshot for that (day, project, transaction), so a re-run for the same
// day replaces rather than accumulates, unlike global.usage_daily's
// running counter — and prunes rows older than transactionLatencyRetention.
func (r *TransactionLatencyRepository) Insert(
	ctx context.Context,
	day time.Time,
	stats []sentryapi.TransactionStat,
) error {
	for _, s := range stats {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO global.transaction_latency_daily
				(day, project, transaction_name, p95_duration_ms, request_count)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (day, project, transaction_name)
			DO UPDATE SET
				p95_duration_ms = EXCLUDED.p95_duration_ms,
				request_count = EXCLUDED.request_count
		`, day, s.Project, s.Transaction, s.P95DurationMs, s.RequestCount); err != nil {
			return err
		}
	}

	_, err := r.db.Exec(ctx,
		`DELETE FROM global.transaction_latency_daily WHERE day < $1`,
		time.Now().Add(-transactionLatencyRetention),
	)
	return err
}

// Trends compares each transaction's average p95 duration over the last
// trendRecentWindow against the trendPriorWindow before that, and returns
// the ones regressing by at least trendMinPctChange, worst first. A
// transaction with no data in either window (not yet snapshotted twice, or
// gone quiet) is excluded rather than reported as a false regression.
func (r *TransactionLatencyRepository) Trends(
	ctx context.Context,
) ([]models.TransactionTrend, error) {
	now := time.Now()
	recentSince := now.Add(-trendRecentWindow)
	priorSince := recentSince.Add(-trendPriorWindow)

	rows, err := r.db.Query(ctx, `
		WITH recent AS (
			SELECT project, transaction_name, AVG(p95_duration_ms) AS avg_p95
			FROM global.transaction_latency_daily
			WHERE day >= $1
			GROUP BY project, transaction_name
		), prior AS (
			SELECT project, transaction_name, AVG(p95_duration_ms) AS avg_p95
			FROM global.transaction_latency_daily
			WHERE day >= $2 AND day < $1
			GROUP BY project, transaction_name
		)
		SELECT
			r.transaction_name, r.project, p.avg_p95, r.avg_p95,
			(r.avg_p95 - p.avg_p95) / p.avg_p95 AS pct_change
		FROM recent r
		JOIN prior p
			ON p.project = r.project AND p.transaction_name = r.transaction_name
		WHERE p.avg_p95 >= $3 AND (r.avg_p95 - p.avg_p95) / p.avg_p95 >= $4
		ORDER BY pct_change DESC
		LIMIT $5
	`, recentSince, priorSince, trendMinPriorP95Ms, trendMinPctChange, trendLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []models.TransactionTrend
	for rows.Next() {
		var t models.TransactionTrend
		if err = rows.Scan(
			&t.Transaction, &t.Project, &t.PriorAvgP95Ms, &t.RecentAvgP95Ms, &t.PctChange,
		); err != nil {
			return nil, err
		}
		trends = append(trends, t)
	}

	return trends, rows.Err()
}

// History returns every (day, project, transaction) row on or after since,
// oldest first — the flat series GetTransactionLatencyHistory returns as-is;
// the client pivots and selects which series to plot. since is caller-bounded
// by windowSince; rows are already pruned to transactionLatencyRetention on
// every Insert, so a since older than that simply returns whatever's stored.
func (r *TransactionLatencyRepository) History(
	ctx context.Context,
	since time.Time,
) ([]models.TransactionLatencyPoint, error) {
	rows, err := r.db.Query(ctx, `
		SELECT day, project, transaction_name, p95_duration_ms, request_count
		FROM global.transaction_latency_daily
		WHERE day >= $1
		ORDER BY day
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.TransactionLatencyPoint
	for rows.Next() {
		var p models.TransactionLatencyPoint
		if err = rows.Scan(
			&p.Day, &p.Project, &p.Transaction, &p.P95DurationMs, &p.RequestCount,
		); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, rows.Err()
}
