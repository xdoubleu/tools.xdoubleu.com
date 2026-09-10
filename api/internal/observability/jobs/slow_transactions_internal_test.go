package jobs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
)

func TestClassifyTransaction(t *testing.T) {
	t.Parallel()

	cases := map[string]transactionClass{
		"GET /games/api/progress":                      transactionClassHTTPHandler,
		"POST /games.v1.GamesService/RefreshSteamGame": transactionClassHTTPHandler,
		"DELETE /books.v1.BooksService/DeleteBook":     transactionClassHTTPHandler,
		"/reading":                              transactionClassFrontend,
		"NextNodeServer.clientComponentLoading": transactionClassFrontend,
		"steam":                                 transactionClassBackgroundJob,
		"poll-feeds":                            transactionClassBackgroundJob,
	}

	for name, want := range cases {
		assert.Equal(t, want, classifyTransaction(name), name)
	}
}

func TestThresholdMsForClass(t *testing.T) {
	t.Parallel()

	assert.InEpsilon(t, 5_000.0, thresholdMsForClass(transactionClassHTTPHandler), 0)
	assert.InEpsilon(t, 5_000.0, thresholdMsForClass(transactionClassFrontend), 0)
	assert.InEpsilon(t, 60_000.0, thresholdMsForClass(transactionClassBackgroundJob), 0)
}

func TestSlowTransactionExcluded(t *testing.T) {
	t.Parallel()

	assert.True(t, slowTransactionExcluded("NextNodeServer.clientComponentLoading"))
	// A long-lived progress WebSocket route is deliberately NOT excluded.
	assert.False(t, slowTransactionExcluded("GET /games/api/progress"))
	assert.False(t, slowTransactionExcluded("/reading"))
}

type fakeTrendsRepo struct {
	trends []models.TransactionTrend
}

func (f fakeTrendsRepo) Trends(context.Context) ([]models.TransactionTrend, error) {
	return f.trends, nil
}

func trend(transaction string, recentP95Ms float64) models.TransactionTrend {
	return models.TransactionTrend{
		Project:        "tools-api",
		Transaction:    transaction,
		PriorAvgP95Ms:  recentP95Ms / 2,
		RecentAvgP95Ms: recentP95Ms,
		PctChange:      1.0,
	}
}

func TestCurrentlySlowTransactions(t *testing.T) {
	t.Parallel()

	repo := fakeTrendsRepo{trends: []models.TransactionTrend{
		trend("GET /games/api/progress", 9_000), // slow
		trend("GET /books.v1.BooksService/Get", 800),
		trend("steam", 30_000), // under job threshold
		trend("NextNodeServer.clientComponentLoading", 61_000),
	}}

	slow, err := currentlySlowTransactions(t.Context(), repo)
	require.NoError(t, err)
	require.Len(t, slow, 1)
	assert.Equal(t, "GET /games/api/progress", slow[0].Transaction)
}
