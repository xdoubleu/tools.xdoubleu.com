package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/repositories"
)

func clearNotifiedIssues(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.notified_issues")
	require.NoError(t, err)
}

func TestNotifiedIssuesExistsAndInsert(t *testing.T) {
	clearNotifiedIssues(t)
	repo := repositories.NewNotifiedIssuesRepository(testDB)

	exists, err := repo.Exists(t.Context(), "sentry:123")
	require.NoError(t, err)
	require.False(t, exists)

	require.NoError(t, repo.Insert(t.Context(), "sentry:123"))

	exists, err = repo.Exists(t.Context(), "sentry:123")
	require.NoError(t, err)
	require.True(t, exists)
}

func TestNotifiedIssuesInsertIsIdempotent(t *testing.T) {
	clearNotifiedIssues(t)
	repo := repositories.NewNotifiedIssuesRepository(testDB)

	require.NoError(t, repo.Insert(t.Context(), "digitalocean:abc"))
	require.NoError(t, repo.Insert(t.Context(), "digitalocean:abc"))

	exists, err := repo.Exists(t.Context(), "digitalocean:abc")
	require.NoError(t, err)
	require.True(t, exists)
}
