package repositories_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/testhelper"
)

// seedAgentUser adds a user with the agent role.
func seedAgentUser(t *testing.T) string {
	t.Helper()
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	id := uuid.NewString()
	email := id + "@agent.invalid"
	_, err := pool.Exec(ctx,
		`INSERT INTO auth.users (id, email, password_hash) VALUES ($1, $2, '!')`,
		id, email)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO global.app_users (id, email, role) VALUES ($1, $2, 'agent')`,
		id, email)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM global.app_users WHERE id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM auth.users WHERE id = $1`, id)
	})
	return id
}

func TestAgentUsersAreLeftOutOfUserLists(t *testing.T) {
	id := seedAgentUser(t)
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	repo := repositories.NewAppUsersRepository(pool)
	ctx := context.Background()

	all, err := repo.GetAll(ctx)
	require.NoError(t, err)
	withAccess, err := repo.GetAllWithAccess(ctx)
	require.NoError(t, err)
	for _, users := range [][]models.User{all, withAccess} {
		for _, u := range users {
			assert.NotEqual(t, id, u.ID)
		}
	}
}

func TestAnAgentUsersRoleCannotBeChanged(t *testing.T) {
	id := seedAgentUser(t)
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	repo := repositories.NewAppUsersRepository(pool)
	ctx := context.Background()

	require.NoError(t, repo.SetRole(ctx, id, models.RoleAdmin))

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAgent, got.Role)
}

func TestAnAgentUserCannotBeGrantedAppAccess(t *testing.T) {
	id := seedAgentUser(t)
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	repo := repositories.NewAppUsersRepository(pool)
	ctx := context.Background()

	require.NoError(t, repo.SetAppAccess(ctx, id, "books", true))

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, got.AppAccess)
}
