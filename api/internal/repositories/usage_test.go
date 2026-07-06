package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

func clearUsage(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.usage_daily")
	require.NoError(t, err)
}

func TestUsageFlushAccumulatesAndGetDaily(t *testing.T) {
	clearUsage(t)
	repo := repositories.NewUsageRepository(testDB)
	today := time.Now()

	require.NoError(t, repo.Flush(t.Context(), []models.UsageEntry{
		{Day: today, App: "books", Endpoint: "root", Count: 2},
		{Day: today, App: "games", Endpoint: "list", Count: 5},
	}))
	// A second flush for the same key must add, not replace.
	require.NoError(t, repo.Flush(t.Context(), []models.UsageEntry{
		{Day: today, App: "books", Endpoint: "root", Count: 3},
	}))

	entries, err := repo.GetDaily(t.Context(), today.AddDate(0, 0, -1))
	require.NoError(t, err)

	counts := map[string]int64{}
	for _, e := range entries {
		counts[e.App+":"+e.Endpoint] = e.Count
	}
	assert.Equal(t, int64(5), counts["books:root"])
	assert.Equal(t, int64(5), counts["games:list"])
}

func TestUsagePruneOlderThan(t *testing.T) {
	clearUsage(t)
	repo := repositories.NewUsageRepository(testDB)

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.usage_daily (day, app, endpoint, count)
		VALUES (now()::date - 500, 'books', 'old', 1)
	`)
	require.NoError(t, err)
	require.NoError(t, repo.Flush(t.Context(), []models.UsageEntry{
		{Day: time.Now(), App: "books", Endpoint: "fresh", Count: 1},
	}))

	require.NoError(t, repo.PruneOlderThan(t.Context(), time.Now().AddDate(0, 0, -400)))

	entries, err := repo.GetDaily(t.Context(), time.Now().AddDate(0, 0, -600))
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotEqual(t, "old", e.Endpoint)
	}
}
