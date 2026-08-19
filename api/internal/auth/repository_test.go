package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/testhelper"
)

// TestRepository_CreateUser exercises the usersStore.CreateUser path — not
// reachable through LocalService (no sign-up flow exists yet, issue #1039
// covers sign-in against pre-seeded users only) but still part of the
// interface's committed surface and worth verifying directly.
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
