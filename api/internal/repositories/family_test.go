package repositories_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/repositories"
)

func clearFamily(t *testing.T, userIDs ...string) {
	t.Helper()
	for _, id := range userIDs {
		_, err := testDB.Exec(t.Context(),
			"DELETE FROM global.family_invites WHERE to_user_id = $1", id)
		require.NoError(t, err)
		_, err = testDB.Exec(t.Context(),
			"DELETE FROM global.family_members WHERE user_id = $1", id)
		require.NoError(t, err)
	}
}

func TestFamilyRepository_EnsureFamilyIsIdempotent(t *testing.T) {
	const userID = "family-repo-user-1"
	clearFamily(t, userID)
	repo := repositories.NewFamilyRepository(testDB)

	_, ok, err := repo.GetFamilyID(t.Context(), userID)
	require.NoError(t, err)
	assert.False(t, ok, "no membership row should exist yet")

	familyID, err := repo.EnsureFamily(t.Context(), userID)
	require.NoError(t, err)

	again, err := repo.EnsureFamily(t.Context(), userID)
	require.NoError(t, err)
	assert.Equal(t, familyID, again, "EnsureFamily should be idempotent")
}

func TestFamilyRepository_InviteAcceptRoundTrip(t *testing.T) {
	const ownerID = "family-repo-owner-1"
	const inviteeID = "family-repo-invitee-1"
	clearFamily(t, ownerID, inviteeID)
	repo := repositories.NewFamilyRepository(testDB)

	_, ok, err := repo.GetInvite(t.Context(), inviteeID)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, repo.Invite(t.Context(), ownerID, inviteeID))

	invite, ok, err := repo.GetInvite(t.Context(), inviteeID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, ownerID, invite.FromUserID)

	joinedFamilyID, err := repo.AcceptInvite(t.Context(), inviteeID)
	require.NoError(t, err)

	ownerFamilyID, ok, err := repo.GetFamilyID(t.Context(), ownerID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, ownerFamilyID, joinedFamilyID)

	members, err := repo.ListMembers(t.Context(), joinedFamilyID)
	require.NoError(t, err)
	assert.Contains(t, members, inviteeID)
	assert.Contains(t, members, ownerID)

	require.NoError(t, repo.Leave(t.Context(), inviteeID))
	_, ok, err = repo.GetFamilyID(t.Context(), inviteeID)
	require.NoError(t, err)
	assert.False(t, ok, "leaving should remove the membership row")
}

func TestFamilyRepository_DeclineInvite(t *testing.T) {
	const ownerID = "family-repo-owner-2"
	const inviteeID = "family-repo-invitee-2"
	clearFamily(t, ownerID, inviteeID)
	repo := repositories.NewFamilyRepository(testDB)

	require.NoError(t, repo.Invite(t.Context(), ownerID, inviteeID))
	require.NoError(t, repo.DeclineInvite(t.Context(), inviteeID))

	_, ok, err := repo.GetInvite(t.Context(), inviteeID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestFamilyRepository_ErrorsOnCanceledContext(t *testing.T) {
	const userID = "family-repo-canceled-ctx"
	clearFamily(t, userID)
	repo := repositories.NewFamilyRepository(testDB)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, _, err := repo.GetFamilyID(ctx, userID)
	require.Error(t, err)

	_, err = repo.EnsureFamily(ctx, userID)
	require.Error(t, err)

	err = repo.Invite(ctx, userID, "someone-else")
	require.Error(t, err)
}
