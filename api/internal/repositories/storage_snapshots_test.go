package repositories_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
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
		OrphanKeys: []string{
			"books/b1/orphan.epub",
			"books/b2/orphan.epub",
		},
		DeletedOrphanSizeBytes: 100,
		DeletedOrphanCount:     1,
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
	assert.Equal(t, int64(1), got.DeletedOrphanCount)
	assert.Equal(t, int64(100), got.DeletedOrphanSizeBytes)
	require.Len(t, got.PrefixBreakdown, 2)
	assert.Equal(t, "books", got.PrefixBreakdown[0].Prefix)
	assert.Equal(
		t,
		[]string{"books/b1/orphan.epub", "books/b2/orphan.epub"},
		got.OrphanKeys,
	)
}

// Production goes through a transaction-mode pooler (simple protocol); the
// default test pool uses the extended one, so this test forces simple.
func TestStorageSnapshotsInsertSimpleProtocol(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	pgxCfg, err := pgxpool.ParseConfig(cfg.DBDsn)
	require.NoError(t, err)
	pgxCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(t.Context(), pgxCfg)
	require.NoError(t, err)
	defer pool.Close()

	clearSnapshots(t)
	repo := repositories.NewStorageSnapshotsRepository(pool)

	require.NoError(t, repo.Insert(t.Context(), sampleSnapshot(time.Now())))

	got, err := repo.Latest(t.Context())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.PrefixBreakdown, 2)
	assert.Equal(t, "books", got.PrefixBreakdown[0].Prefix)
}

// Valid JSON of the wrong shape is the only way to reach scanSnapshot's
// unmarshal error.
func TestStorageSnapshotsLatestOrphanKeysTypeMismatch(t *testing.T) {
	clearSnapshots(t)
	repo := repositories.NewStorageSnapshotsRepository(testDB)

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.storage_snapshots (
			scanned_at, total_size_bytes, object_count,
			orphan_size_bytes, orphan_count,
			stale_upload_size_bytes, stale_upload_count, prefix_breakdown,
			orphan_keys
		) VALUES (now(), 0, 0, 0, 0, 0, 0, '[]', '{"not":"an array"}')
	`)
	require.NoError(t, err)

	_, err = repo.Latest(t.Context())
	assert.Error(t, err)
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
