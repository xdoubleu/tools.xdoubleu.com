package repositories_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

//nolint:gochecknoglobals //needed for tests
var testDB postgres.DB

func TestMain(m *testing.M) {
	cfg := testhelper.NewTestConfig()
	postgresDB := testhelper.ConnectTestDB(cfg.DBDsn)
	testDB = postgresDB

	// Mirrors cmd/api/migrations/00001_init.sql, 00005_observability.sql, and
	// 00007_profile_shares_per_app.sql so these tests can run before the
	// cmd/api package has applied the global migrations.
	ctx := context.Background()
	stmts := []string{
		"CREATE SCHEMA IF NOT EXISTS global",
		`CREATE TABLE IF NOT EXISTS global.app_users (
			id           TEXT PRIMARY KEY,
			email        TEXT NOT NULL,
			last_seen    TIMESTAMPTZ NOT NULL DEFAULT now(),
			role         TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			display_name TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS global.profile_shares (
			user_id TEXT NOT NULL,
			app TEXT NOT NULL CHECK (app IN ('books', 'games')),
			token TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, app)
		)`,
		`CREATE TABLE IF NOT EXISTS global.job_runs (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			job_id TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			duration_ms BIGINT NOT NULL,
			success BOOLEAN NOT NULL,
			error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS global.usage_daily (
			day DATE NOT NULL,
			app TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			count BIGINT NOT NULL,
			PRIMARY KEY (day, app, endpoint)
		)`,
		`CREATE TABLE IF NOT EXISTS global.storage_snapshots (
			id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			scanned_at TIMESTAMPTZ NOT NULL,
			total_size_bytes BIGINT NOT NULL,
			object_count BIGINT NOT NULL,
			orphan_size_bytes BIGINT NOT NULL,
			orphan_count BIGINT NOT NULL,
			stale_upload_size_bytes BIGINT NOT NULL,
			stale_upload_count BIGINT NOT NULL,
			prefix_breakdown JSONB NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := postgresDB.Exec(ctx, stmt); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}

func clearJobRuns(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.job_runs")
	require.NoError(t, err)
}

func TestJobRunsInsertAndListRecent(t *testing.T) {
	clearJobRuns(t)
	repo := repositories.NewJobRunsRepository(testDB)

	now := time.Now()
	require.NoError(t, repo.Insert(t.Context(), models.JobRun{
		JobID:      "steam",
		StartedAt:  now.Add(-time.Hour),
		DurationMs: 1200,
		Success:    true,
		Error:      "",
	}))
	require.NoError(t, repo.Insert(t.Context(), models.JobRun{
		JobID:      "steam",
		StartedAt:  now,
		DurationMs: 800,
		Success:    false,
		Error:      "steam api unreachable",
	}))

	runs, err := repo.ListRecent(t.Context(), now.Add(-24*time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, "steam", runs[0].JobID)
	assert.False(t, runs[0].Success)
	assert.Equal(t, "steam api unreachable", runs[0].Error)
	assert.True(t, runs[1].Success)
	assert.Empty(t, runs[1].Error)
}

func TestJobRunsStats(t *testing.T) {
	clearJobRuns(t)
	repo := repositories.NewJobRunsRepository(testDB)

	now := time.Now()
	for i, ok := range []bool{true, true, false} {
		errMsg := ""
		if !ok {
			errMsg = "boom"
		}
		require.NoError(t, repo.Insert(t.Context(), models.JobRun{
			JobID:      "todos-archive",
			StartedAt:  now.Add(time.Duration(-i) * time.Hour),
			DurationMs: int64(100 * (i + 1)),
			Success:    ok,
			Error:      errMsg,
		}))
	}

	stats, err := repo.Stats(t.Context(), now.Add(-24*time.Hour))
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, "todos-archive", stats[0].JobID)
	assert.Equal(t, int64(3), stats[0].TotalRuns)
	assert.Equal(t, int64(1), stats[0].FailedRuns)
	assert.Equal(t, int64(200), stats[0].AvgDurationMs)
	assert.WithinDuration(t, now, stats[0].LastRunAt, time.Second)
}

func TestJobRunsInsertPrunesOldRows(t *testing.T) {
	clearJobRuns(t)
	repo := repositories.NewJobRunsRepository(testDB)

	_, err := testDB.Exec(t.Context(), `
		INSERT INTO global.job_runs (job_id, started_at, duration_ms, success)
		VALUES ('ancient', now() - INTERVAL '91 days', 1, TRUE)
	`)
	require.NoError(t, err)

	require.NoError(t, repo.Insert(t.Context(), models.JobRun{
		JobID:      "fresh",
		StartedAt:  time.Now(),
		DurationMs: 1,
		Success:    true,
		Error:      "",
	}))

	var count int
	err = testDB.QueryRow(
		t.Context(),
		"SELECT COUNT(*) FROM global.job_runs WHERE job_id = 'ancient'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count)
}
