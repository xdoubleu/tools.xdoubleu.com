package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/internal/repositories"
)

const shareUserID = "cccccccc-1111-2222-3333-444444444444"

func clearProfileShares(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(),
		"DELETE FROM global.profile_shares WHERE user_id = $1", shareUserID)
	require.NoError(t, err)
}

func TestProfileSharesRoundTrip(t *testing.T) {
	clearProfileShares(t)
	repo := repositories.NewProfileSharesRepository(testDB)

	_, err := repo.Get(t.Context(), shareUserID)
	assert.ErrorIs(t, err, database.ErrResourceNotFound, "no share should exist yet")

	created, err := repo.Upsert(t.Context(), shareUserID, "repo-test-token")
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "repo-test-token", created.Token)
	assert.False(t, created.CreatedAt.IsZero())

	share, err := repo.Get(t.Context(), shareUserID)
	require.NoError(t, err)
	require.NotNil(t, share)
	assert.Equal(t, "repo-test-token", share.Token)

	owner, err := repo.GetUserIDByToken(t.Context(), "repo-test-token")
	require.NoError(t, err)
	assert.Equal(t, shareUserID, owner)

	// Upsert replaces the token; the old one stops resolving.
	replaced, err := repo.Upsert(t.Context(), shareUserID, "repo-test-token-2")
	require.NoError(t, err)
	assert.Equal(t, "repo-test-token-2", replaced.Token)

	_, err = repo.GetUserIDByToken(t.Context(), "repo-test-token")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)

	require.NoError(t, repo.Delete(t.Context(), shareUserID))

	_, err = repo.Get(t.Context(), shareUserID)
	assert.ErrorIs(t, err, database.ErrResourceNotFound,
		"share should be gone after delete")

	_, err = repo.GetUserIDByToken(t.Context(), "repo-test-token-2")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestProfileSharesGetUserIDByToken_Unknown(t *testing.T) {
	repo := repositories.NewProfileSharesRepository(testDB)
	_, err := repo.GetUserIDByToken(t.Context(), "no-such-token")
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}
