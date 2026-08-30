package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

func clearDBSizeSamples(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.db_size_samples")
	require.NoError(t, err)
}

func TestDBSizeSamplesInsertBatchAndPrune(t *testing.T) {
	clearDBSizeSamples(t)
	repo := repositories.NewDBSizeSamplesRepository(testDB)

	old := time.Now().AddDate(0, 0, -120)
	require.NoError(t, repo.InsertBatch(t.Context(), old, []models.TableSizeSample{
		{SchemaName: "public", TableName: "old_table", SizeBytes: 100},
	}))
	require.NoError(
		t,
		repo.InsertBatch(t.Context(), time.Now(), []models.TableSizeSample{
			{SchemaName: "public", TableName: "old_table", SizeBytes: 200},
		}),
	)

	require.NoError(t, repo.PruneOlderThan(t.Context(), time.Now().AddDate(0, 0, -90)))

	var count int
	require.NoError(t, testDB.QueryRow(
		t.Context(), "SELECT count(*) FROM global.db_size_samples",
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestDBSizeSamplesGrowth(t *testing.T) {
	clearDBSizeSamples(t)
	repo := repositories.NewDBSizeSamplesRepository(testDB)
	now := time.Now()

	require.NoError(
		t,
		repo.InsertBatch(t.Context(), now.Add(-time.Hour), []models.TableSizeSample{
			{SchemaName: "public", TableName: "growing", SizeBytes: 1000},
			{SchemaName: "public", TableName: "single_sample", SizeBytes: 500},
		}),
	)
	require.NoError(t, repo.InsertBatch(t.Context(), now, []models.TableSizeSample{
		{SchemaName: "public", TableName: "growing", SizeBytes: 1500},
	}))

	growth, err := repo.Growth(t.Context(), now.Add(-2*time.Hour))
	require.NoError(t, err)
	require.Len(t, growth, 1)
	assert.Equal(t, "growing", growth[0].TableName)
	assert.EqualValues(t, 1500, growth[0].CurrentSizeBytes)
	assert.EqualValues(t, 1000, growth[0].EarliestSizeBytes)
	assert.EqualValues(t, 500, growth[0].DeltaBytes)
	assert.InDelta(t, 0.5, growth[0].PctChange, 0.001)
}

func TestDBSizeSamplesHistory(t *testing.T) {
	clearDBSizeSamples(t)
	repo := repositories.NewDBSizeSamplesRepository(testDB)
	now := time.Now()

	require.NoError(
		t,
		repo.InsertBatch(t.Context(), now.AddDate(0, 0, -10), []models.TableSizeSample{
			{SchemaName: "public", TableName: "outside_window", SizeBytes: 1},
		}),
	)
	require.NoError(
		t,
		repo.InsertBatch(t.Context(), now.Add(-time.Hour), []models.TableSizeSample{
			{SchemaName: "public", TableName: "a", SizeBytes: 1000},
			{SchemaName: "public", TableName: "b", SizeBytes: 500},
		}),
	)
	require.NoError(t, repo.InsertBatch(t.Context(), now, []models.TableSizeSample{
		{SchemaName: "public", TableName: "a", SizeBytes: 1200},
		{SchemaName: "public", TableName: "b", SizeBytes: 800},
	}))

	history, err := repo.History(t.Context(), now.Add(-2*time.Hour))
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.EqualValues(t, 1500, history[0].TotalSizeBytes)
	assert.EqualValues(t, 2000, history[1].TotalSizeBytes)
	assert.True(t, history[0].SampledAt.Before(history[1].SampledAt))
}

func TestDBSizeSamplesHistory_QueryError(t *testing.T) {
	repo := repositories.NewDBSizeSamplesRepository(testDB)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := repo.History(ctx, time.Now())
	require.Error(t, err)
}

func TestDBSizeSamplesPerTableHistoryReturnsRowsSinceOrdered(t *testing.T) {
	clearDBSizeSamples(t)
	repo := repositories.NewDBSizeSamplesRepository(testDB)
	now := time.Now()

	require.NoError(
		t,
		repo.InsertBatch(t.Context(), now.AddDate(0, 0, -40), []models.TableSizeSample{
			{SchemaName: "public", TableName: "old", SizeBytes: 100},
		}),
	)
	require.NoError(
		t,
		repo.InsertBatch(
			t.Context(),
			now.Add(-2*24*time.Hour),
			[]models.TableSizeSample{
				{SchemaName: "public", TableName: "a", SizeBytes: 1000},
				{SchemaName: "reading", TableName: "b", SizeBytes: 500},
			},
		),
	)
	require.NoError(
		t,
		repo.InsertBatch(
			t.Context(),
			now.Add(-1*24*time.Hour),
			[]models.TableSizeSample{
				{SchemaName: "public", TableName: "a", SizeBytes: 1200},
			},
		),
	)

	points, err := repo.PerTableHistory(t.Context(), now.Add(-10*24*time.Hour))
	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.Equal(t, "public", points[0].SchemaName)
	assert.Equal(t, "a", points[0].TableName)
	assert.EqualValues(t, 1000, points[0].SizeBytes)
	assert.Equal(t, "reading", points[1].SchemaName)
	assert.Equal(t, "b", points[1].TableName)
	assert.EqualValues(t, 500, points[1].SizeBytes)
	assert.Equal(t, "a", points[2].TableName)
	assert.EqualValues(t, 1200, points[2].SizeBytes)
	assert.True(
		t,
		points[0].Day.Before(points[2].Day) || points[0].Day.Equal(points[2].Day),
	)
}

func TestDBSizeSamplesPerTableHistory_QueryError(t *testing.T) {
	repo := repositories.NewDBSizeSamplesRepository(testDB)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := repo.PerTableHistory(ctx, time.Now())
	require.Error(t, err)
}

func TestDBSizeSamplesGrowth_ExcludesOutsideWindow(t *testing.T) {
	clearDBSizeSamples(t)
	repo := repositories.NewDBSizeSamplesRepository(testDB)
	now := time.Now()

	require.NoError(
		t,
		repo.InsertBatch(t.Context(), now.AddDate(0, 0, -10), []models.TableSizeSample{
			{SchemaName: "public", TableName: "stale", SizeBytes: 100},
		}),
	)
	require.NoError(t, repo.InsertBatch(t.Context(), now, []models.TableSizeSample{
		{SchemaName: "public", TableName: "stale", SizeBytes: 200},
	}))

	growth, err := repo.Growth(t.Context(), now.AddDate(0, 0, -1))
	require.NoError(t, err)
	assert.Empty(t, growth)
}
