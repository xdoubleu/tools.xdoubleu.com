package repositories_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

func clearSnapshots(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.storage_snapshots")
	require.NoError(t, err)
}

func sampleSnapshot(scannedAt time.Time) models.StorageSnapshot {
	return models.StorageSnapshot{
		ScannedAt:            scannedAt,
		TotalSizeBytes:       1000,
		ObjectCount:          10,
		OrphanSizeBytes:      200,
		OrphanCount:          2,
		StaleUploadSizeBytes: 50,
		StaleUploadCount:     1,
		PrefixBreakdown: []models.PrefixStat{
			{Prefix: "books", SizeBytes: 900, Count: 8},
			{Prefix: "users", SizeBytes: 100, Count: 2},
		},
	}
}

func TestStorageSnapshotsInsertAndLatest(t *testing.T) {
	clearSnapshots(t)
	repo := repositories.NewStorageSnapshotsRepository(testDB)

	now := time.Now()
	require.NoError(t, repo.Insert(t.Context(), sampleSnapshot(now.Add(-time.Hour))))
	latest := sampleSnapshot(now)
	latest.TotalSizeBytes = 2000
	require.NoError(t, repo.Insert(t.Context(), latest))

	got, err := repo.Latest(t.Context())
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(2000), got.TotalSizeBytes)
	assert.Equal(t, int64(2), got.OrphanCount)
	require.Len(t, got.PrefixBreakdown, 2)
	assert.Equal(t, "books", got.PrefixBreakdown[0].Prefix)
}

func TestStorageSnapshotsHistory(t *testing.T) {
	clearSnapshots(t)
	repo := repositories.NewStorageSnapshotsRepository(testDB)

	now := time.Now()
	require.NoError(t, repo.Insert(t.Context(), sampleSnapshot(now.Add(-48*time.Hour))))
	require.NoError(t, repo.Insert(t.Context(), sampleSnapshot(now.Add(-time.Hour))))

	hist, err := repo.History(t.Context(), now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Len(t, hist, 1)
	assert.WithinDuration(t, now.Add(-time.Hour), hist[0].ScannedAt, time.Second)
}
