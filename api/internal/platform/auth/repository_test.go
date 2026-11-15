package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/platform/auth"
)

// CreateUser isn't reachable through LocalService (no sign-up flow).
func TestRepository_CreateUser(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	repo := auth.NewRepository(db)

	email := uuid.NewString() + "@example.com"
	user, err := repo.CreateUser(context.Background(), email, "some-bcrypt-hash")
	require.NoError(t, err)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, "some-bcrypt-hash", user.PasswordHash)
	assert.NotEmpty(t, user.ID)

	got, err := repo.GetUserByEmail(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
}

func TestRepository_GetTOTPFactor_NotFound(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	repo := auth.NewRepository(db)

	_, err := repo.GetTOTPFactor(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestRepository_GetUnusedRecoveryCodes_Empty(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	repo := auth.NewRepository(db)

	codes, err := repo.GetUnusedRecoveryCodes(context.Background(), uuid.NewString())
	require.NoError(t, err)
	assert.Empty(t, codes)
}

func TestRepository_OIDCSubject(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	repo := auth.NewRepository(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, uuid.NewString()+"@example.com", "hash")
	require.NoError(t, err)
	subject := uuid.NewString()

	_, err = repo.GetUserByOIDCSubject(ctx, subject)
	require.Error(t, err)

	require.NoError(t, repo.SetOIDCSubject(ctx, user.ID, subject))
	got, err := repo.GetUserByOIDCSubject(ctx, subject)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)

	other, err := repo.CreateUser(ctx, uuid.NewString()+"@example.com", "hash")
	require.NoError(t, err)
	require.Error(t, repo.SetOIDCSubject(ctx, other.ID, subject), "subjects are unique")
}

func TestRepository_OIDCUserLifecycle(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	repo := auth.NewRepository(db)
	ctx := context.Background()

	subject := uuid.NewString()
	user, err := repo.CreateOIDCUser(ctx, uuid.NewString()+"@example.com", subject)
	require.NoError(t, err)
	assert.Equal(t, subject, user.OIDCSubject)
	assert.False(t, user.Deactivated)

	_, err = repo.CreateOIDCUser(ctx, user.Email, uuid.NewString())
	require.Error(t, err, "emails are unique")

	require.NoError(t, repo.SetDeactivated(ctx, user.ID))
	require.NoError(t, repo.SetDeactivated(ctx, user.ID), "repeatable")
	got, err := repo.GetUserByOIDCSubject(ctx, subject)
	require.NoError(t, err)
	assert.True(t, got.Deactivated)

	require.NoError(t, repo.DeleteUser(ctx, user.ID))
	_, err = repo.GetUserByID(ctx, user.ID)
	require.Error(t, err)
}

func TestRepository_ListImportCandidates(t *testing.T) {
	db := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(db.Close)
	repo := auth.NewRepository(db)
	ctx := context.Background()

	add := func(role string, subject bool) *auth.User {
		user, err := repo.CreateUser(ctx, uuid.NewString()+"@example.com", "hash")
		require.NoError(t, err)
		_, err = db.Exec(ctx, `
			INSERT INTO global.app_users (id, email, role, display_name)
			VALUES ($1, $2, $3, 'Name')`, user.ID, user.Email, role)
		require.NoError(t, err)
		if subject {
			require.NoError(t, repo.SetOIDCSubject(ctx, user.ID, uuid.NewString()))
		}
		return user
	}
	person := add("user", false)
	agent := add("agent", false)
	linked := add("user", true)
	withTOTP := add("admin", false)
	factor, err := repo.CreateTOTPFactor(ctx, withTOTP.ID, "sealed")
	require.NoError(t, err)
	require.NoError(t, repo.VerifyTOTPFactor(ctx, factor))

	got, err := repo.ListImportCandidates(ctx)
	require.NoError(t, err)

	byID := map[string]auth.ImportCandidate{}
	for _, c := range got {
		byID[c.UserID] = c
	}
	assert.Contains(t, byID, person.ID)
	assert.Equal(t, "Name", byID[person.ID].DisplayName)
	assert.Empty(t, byID[person.ID].SealedTOTP)
	assert.NotContains(t, byID, agent.ID)
	assert.NotContains(t, byID, linked.ID)
	assert.Equal(t, "sealed", byID[withTOTP.ID].SealedTOTP)
}
