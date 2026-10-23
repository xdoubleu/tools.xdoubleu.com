package books_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyMigrations_ConvergedDBKeepsSchema: re-running migrations on a
// migrated database leaves the books schema alone, so serving replicas never
// see it missing mid-boot.
func TestApplyMigrations_ConvergedDBKeepsSchema(t *testing.T) {
	pool, ok := testDB.(*pgxpool.Pool)
	require.True(t, ok)

	before := booksSchemaXmin(t)
	require.NoError(t, testApp.ApplyMigrations(t.Context(), pool))

	// Any ALTER SCHEMA rewrites the pg_namespace row, which moves its xmin.
	assert.Equal(t, before, booksSchemaXmin(t))
}

func TestApplyMigrations_NoOrphanFeedTables(t *testing.T) {
	for _, table := range []string{"books.feeds", "books.feed_items"} {
		var exists bool
		err := testDB.QueryRow(
			t.Context(),
			"SELECT to_regclass($1) IS NOT NULL",
			table,
		).Scan(&exists)
		require.NoError(t, err)
		assert.False(t, exists, table)
	}
}

func booksSchemaXmin(t *testing.T) string {
	t.Helper()
	var xmin string
	err := testDB.QueryRow(
		t.Context(),
		"SELECT xmin::text FROM pg_namespace WHERE nspname = 'books'",
	).Scan(&xmin)
	require.NoError(t, err)
	return xmin
}
