package account_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/testhelper"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/account"
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
	_, err = pool.Exec(ctx, `
		INSERT INTO global.agent_accounts (user_id, client_id, name, created_by)
		VALUES ($1, $2, 'seeded', 'test')`, id, "agent-"+id)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM global.agent_accounts WHERE user_id = $1`, id)
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM global.app_access WHERE user_id = $1`, id)
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
	repo := account.NewAppUsersRepository(pool)
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
	repo := account.NewAppUsersRepository(pool)
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
	repo := account.NewAppUsersRepository(pool)
	ctx := context.Background()

	require.NoError(t, repo.SetAppAccess(ctx, id, "books", true))

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, got.AppAccess)
}

func TestAStandingAgentIsGrantedReadAndWriteAccess(t *testing.T) {
	id := seedAgentUser(t)
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	repo := account.NewAppUsersRepository(pool)
	ctx := context.Background()

	ok, err := repo.SetAgentAppAccess(ctx, id, "books", account.AgentRead)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = repo.SetAgentAppAccess(ctx, id, "feeds", account.AgentWrite)
	require.NoError(t, err)
	require.True(t, ok)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"books", "feeds"}, got.AppAccess)
	assert.Equal(t, []string{"feeds"}, got.WriteAccess)

	ok, err = repo.SetAgentAppAccess(ctx, id, "books", account.AgentWrite)
	require.NoError(t, err)
	require.True(t, ok)
	got, err = repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"books", "feeds"}, got.WriteAccess)

	ok, err = repo.SetAgentAppAccess(ctx, id, "books", account.AgentNoAccess)
	require.NoError(t, err)
	require.True(t, ok)
	got, err = repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"feeds"}, got.AppAccess)
}

func TestAgentAccessIsRefusedForARevokedOrJobAccount(t *testing.T) {
	revoked := seedAgentUser(t)
	job := seedAgentUser(t)
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	repo := account.NewAppUsersRepository(pool)
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`UPDATE global.agent_accounts SET revoked_at = now() WHERE user_id = $1`,
		revoked)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`UPDATE global.agent_accounts SET job_id = $2, task_id = $2
		 WHERE user_id = $1`, job, uuid.New())
	require.NoError(t, err)

	for _, id := range []string{revoked, job} {
		ok, grantErr := repo.SetAgentAppAccess(ctx, id, "books", account.AgentWrite)
		require.NoError(t, grantErr)
		assert.False(t, ok)
		got, getErr := repo.GetByID(ctx, id)
		require.NoError(t, getErr)
		assert.Empty(t, got.AppAccess)
	}
}

func TestAgentAccessIsRefusedForAPerson(t *testing.T) {
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	repo := account.NewAppUsersRepository(pool)
	ctx := context.Background()
	id := uuid.NewString()
	require.NoError(t, repo.Upsert(ctx, id, id+"@example.com"))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM global.app_users WHERE id = $1`, id)
	})

	ok, err := repo.SetAgentAppAccess(ctx, id, "books", account.AgentWrite)
	require.NoError(t, err)
	assert.False(t, ok)

	got, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, got.AppAccess)
}
