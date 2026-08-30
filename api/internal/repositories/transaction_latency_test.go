package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
)

func clearTransactionLatency(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.transaction_latency_daily")
	require.NoError(t, err)
}

func TestTransactionLatencyInsertUpserts(t *testing.T) {
	clearTransactionLatency(t)
	repo := repositories.NewTransactionLatencyRepository(testDB)

	day := time.Now().Truncate(24 * time.Hour)
	stats := []sentryapi.TransactionStat{
		{
			Transaction:   "GET /api/books",
			Project:       "proj",
			P95DurationMs: 100,
			RequestCount:  10,
		},
	}
	require.NoError(t, repo.Insert(t.Context(), day, stats))

	// Re-inserting the same day/project/transaction replaces, not adds.
	stats[0].P95DurationMs = 200
	stats[0].RequestCount = 20
	require.NoError(t, repo.Insert(t.Context(), day, stats))

	var p95 float64
	var count int64
	err := testDB.QueryRow(t.Context(), `
		SELECT p95_duration_ms, request_count FROM global.transaction_latency_daily
		WHERE day = $1 AND project = 'proj' AND transaction_name = 'GET /api/books'
	`, day).Scan(&p95, &count)
	require.NoError(t, err)
	assert.InEpsilon(t, 200.0, p95, 0.001)
	assert.Equal(t, int64(20), count)

	var rowCount int
	require.NoError(t, testDB.QueryRow(t.Context(),
		"SELECT COUNT(*) FROM global.transaction_latency_daily",
	).Scan(&rowCount))
	assert.Equal(t, 1, rowCount, "must not have inserted a duplicate row")
}

func TestTransactionLatencyInsertPrunesOldRows(t *testing.T) {
	clearTransactionLatency(t)
	repo := repositories.NewTransactionLatencyRepository(testDB)

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.transaction_latency_daily
			(day, project, transaction_name, p95_duration_ms, request_count)
		VALUES (now() - INTERVAL '91 days', 'proj', 'ancient', 1, 1)
	`)
	require.NoError(t, err)

	require.NoError(t, repo.Insert(t.Context(), time.Now(), []sentryapi.TransactionStat{
		{Transaction: "fresh", Project: "proj", P95DurationMs: 1, RequestCount: 1},
	}))

	var count int
	err = testDB.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM global.transaction_latency_daily
		WHERE transaction_name = 'ancient'
	`).Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestTransactionLatencyTrendsFlagsRegression(t *testing.T) {
	clearTransactionLatency(t)
	repo := repositories.NewTransactionLatencyRepository(testDB)
	now := time.Now()

	// "GET /slow" regressed: prior week averaged 100ms, recent week 200ms
	// (+100%, well above the 20% threshold and the 50ms floor).
	seedDay(t, now.Add(-3*24*time.Hour), "GET /slow", 200)
	seedDay(t, now.Add(-10*24*time.Hour), "GET /slow", 100)

	// "GET /stable" did not regress: flat at 150ms both weeks.
	seedDay(t, now.Add(-3*24*time.Hour), "GET /stable", 150)
	seedDay(t, now.Add(-10*24*time.Hour), "GET /stable", 150)

	// "GET /tiny" jumped 100% but from a near-zero baseline (below the
	// 50ms floor) — must be excluded as noise.
	seedDay(t, now.Add(-3*24*time.Hour), "GET /tiny", 10)
	seedDay(t, now.Add(-10*24*time.Hour), "GET /tiny", 5)

	// "GET /new" only has recent data (no prior window) — must be excluded,
	// not reported as an infinite regression.
	seedDay(t, now.Add(-3*24*time.Hour), "GET /new", 500)

	trends, err := repo.Trends(t.Context())
	require.NoError(t, err)
	require.Len(t, trends, 1)
	assert.Equal(t, "GET /slow", trends[0].Transaction)
	assert.Equal(t, "proj", trends[0].Project)
	assert.InEpsilon(t, 100.0, trends[0].PriorAvgP95Ms, 0.001)
	assert.InEpsilon(t, 200.0, trends[0].RecentAvgP95Ms, 0.001)
	assert.InEpsilon(t, 1.0, trends[0].PctChange, 0.001)
}

func TestTransactionLatencyHistoryReturnsRowsSinceOrdered(t *testing.T) {
	clearTransactionLatency(t)
	repo := repositories.NewTransactionLatencyRepository(testDB)
	now := time.Now()

	seedDay(t, now.Add(-40*24*time.Hour), "GET /old", 50)
	seedDay(t, now.Add(-2*24*time.Hour), "GET /recent-a", 100)
	seedDay(t, now.Add(-1*24*time.Hour), "GET /recent-b", 150)

	points, err := repo.History(t.Context(), now.Add(-10*24*time.Hour))
	require.NoError(t, err)
	require.Len(t, points, 2)
	assert.Equal(t, "GET /recent-a", points[0].Transaction)
	assert.Equal(t, "GET /recent-b", points[1].Transaction)
	assert.Equal(t, "proj", points[0].Project)
	assert.InEpsilon(t, 100.0, points[0].P95DurationMs, 0.001)
	assert.Equal(t, int64(1), points[0].RequestCount)
}

func seedDay(t *testing.T, day time.Time, transaction string, p95Ms float64) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.transaction_latency_daily
			(day, project, transaction_name, p95_duration_ms, request_count)
		VALUES ($1, 'proj', $2, $3, 1)
	`, day, transaction, p95Ms)
	require.NoError(t, err)
}
