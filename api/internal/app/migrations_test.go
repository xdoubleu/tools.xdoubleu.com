package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/app/migrationsfixture"
	"tools.xdoubleu.com/internal/kit/testhelper"
)

// Timestamp versions sort after sequential ones, so they apply last, and
// re-running is a no-op.
func TestApplyMigrationsFromFS_TimestampAfterSequential(t *testing.T) {
	cfg := testhelper.NewTestConfig()
	pool := testhelper.ConnectTestDB(cfg.DBDsn)
	ctx := t.Context()
	const schema = "migfixture"

	_, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(
			t.Context(),
			"DROP SCHEMA IF EXISTS "+schema+" CASCADE",
		)
	})

	base := newTestBase(ctx)
	for range 2 {
		require.NoError(
			t,
			base.ApplyMigrationsFromFS(ctx, pool, migrationsfixture.FS, schema),
		)
	}

	rows, err := pool.Query(
		ctx,
		"SELECT version_id FROM "+schema+
			".goose_db_version WHERE version_id > 0 ORDER BY id",
	)
	require.NoError(t, err)
	defer rows.Close()

	var versions []int64
	for rows.Next() {
		var v int64
		require.NoError(t, rows.Scan(&v))
		versions = append(versions, v)
	}
	assert.Equal(t, []int64{1, 2, 20260101000000}, versions)
}
