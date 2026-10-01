package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

const (
	// transactionLatencyRetention: only a recent-vs-prior comparison reads it.
	transactionLatencyRetention = 90 * 24 * time.Hour

	// Trends compares the last 7 days with the 7 before.
	trendRecentWindow = 7 * 24 * time.Hour
	trendPriorWindow  = 7 * 24 * time.Hour
	// trendMinPriorP95Ms ignores near-zero baselines.
	trendMinPriorP95Ms = 50.0
	// trendMinWindowRequests (~7/day) keeps a handful of page loads, whose p95
	// is just their slowest one, from reading as a regression.
	trendMinWindowRequests = 50
	trendMinPctChange      = 0.20
	trendLimit             = 20
)

type TransactionLatencyRepository struct {
	db postgres.DB
}

func NewTransactionLatencyRepository(db postgres.DB) *TransactionLatencyRepository {
	return &TransactionLatencyRepository{db: db}
}

// Insert upserts one day's stats (a re-run replaces) and prunes old rows.
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

// Trends returns transactions whose request-weighted average p95 rose by at
// least trendMinPctChange between the two windows, worst first; ones missing
// either window or under trendMinWindowRequests in either are excluded.
func (r *TransactionLatencyRepository) Trends(
	ctx context.Context,
) ([]models.TransactionTrend, error) {
	now := time.Now()
	recentSince := now.Add(-trendRecentWindow)
	priorSince := recentSince.Add(-trendPriorWindow)

	rows, err := r.db.Query(ctx, `
		WITH recent AS (
			SELECT project, transaction_name,
				SUM(p95_duration_ms * request_count) / SUM(request_count) AS avg_p95
			FROM global.transaction_latency_daily
			WHERE day >= $1
			GROUP BY project, transaction_name
			HAVING SUM(request_count) >= $6
		), prior AS (
			SELECT project, transaction_name,
				SUM(p95_duration_ms * request_count) / SUM(request_count) AS avg_p95
			FROM global.transaction_latency_daily
			WHERE day >= $2 AND day < $1
			GROUP BY project, transaction_name
			HAVING SUM(request_count) >= $6
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
	`, recentSince, priorSince, trendMinPriorP95Ms, trendMinPctChange, trendLimit,
		trendMinWindowRequests)
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
