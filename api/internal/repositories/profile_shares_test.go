package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/internal/models"
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

	_, err := repo.Get(t.Context(), shareUserID, models.ProfileAppBooks)
	assert.ErrorIs(t, err, database.ErrResourceNotFound, "no share should exist yet")

	created, err := repo.Upsert(
		t.Context(),
		shareUserID,
		models.ProfileAppBooks,
		"repo-test-token",
	)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "repo-test-token", created.Token)
	assert.False(t, created.CreatedAt.IsZero())

	share, err := repo.Get(t.Context(), shareUserID, models.ProfileAppBooks)
	require.NoError(t, err)
	require.NotNil(t, share)
	assert.Equal(t, "repo-test-token", share.Token)

	owner, _, err := repo.ResolveToken(
		t.Context(),
		"repo-test-token",
		models.ProfileAppBooks,
	)
	require.NoError(t, err)
	assert.Equal(t, shareUserID, owner)

	// A token resolved against the wrong app is unknown, even though it
	// belongs to the same user.
	_, _, err = repo.ResolveToken(
		t.Context(),
		"repo-test-token",
		models.ProfileAppGames,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)

	// Upsert replaces the token; the old one stops resolving.
	replaced, err := repo.Upsert(
		t.Context(),
		shareUserID,
		models.ProfileAppBooks,
		"repo-test-token-2",
	)
	require.NoError(t, err)
	assert.Equal(t, "repo-test-token-2", replaced.Token)

	_, _, err = repo.ResolveToken(
		t.Context(),
		"repo-test-token",
		models.ProfileAppBooks,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)

	require.NoError(t, repo.Delete(t.Context(), shareUserID, models.ProfileAppBooks))

	_, err = repo.Get(t.Context(), shareUserID, models.ProfileAppBooks)
	assert.ErrorIs(t, err, database.ErrResourceNotFound,
		"share should be gone after delete")

	_, _, err = repo.ResolveToken(
		t.Context(),
		"repo-test-token-2",
		models.ProfileAppBooks,
	)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestProfileSharesIndependentPerApp(t *testing.T) {
	clearProfileShares(t)
	repo := repositories.NewProfileSharesRepository(testDB)

	_, err := repo.Upsert(
		t.Context(),
		shareUserID,
		models.ProfileAppBooks,
		"books-token",
	)
	require.NoError(t, err)
	_, err = repo.Upsert(
		t.Context(),
		shareUserID,
		models.ProfileAppGames,
		"games-token",
	)
	require.NoError(t, err)

	// Deleting the books share must not affect the games share.
	require.NoError(t, repo.Delete(t.Context(), shareUserID, models.ProfileAppBooks))

	_, err = repo.Get(t.Context(), shareUserID, models.ProfileAppBooks)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)

	gamesShare, err := repo.Get(t.Context(), shareUserID, models.ProfileAppGames)
	require.NoError(t, err)
	assert.Equal(t, "games-token", gamesShare.Token)
}

func TestProfileSharesResolveToken_Unknown(t *testing.T) {
	repo := repositories.NewProfileSharesRepository(testDB)
	_, _, err := repo.ResolveToken(t.Context(), "no-such-token", models.ProfileAppBooks)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}
