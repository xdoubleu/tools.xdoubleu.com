package agentaccounts_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"tools.xdoubleu.com/internal/agentaccounts"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/testhelper"
)

func newStore(t *testing.T, onRevoke func()) (*agentaccounts.Store, postgres.DB) {
	t.Helper()
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	return agentaccounts.NewStore(pool, onRevoke), pool
}

func TestCreateStoresAnAgentUserClientAndAccount(t *testing.T) {
	store, db := newStore(t, nil)
	ctx := context.Background()

	account, secret, err := store.Create(ctx, "human:creator", "Night shift")
	require.NoError(t, err)
	assert.Equal(t, "Night shift", account.Name)
	assert.Equal(t, "human:creator", account.CreatedBy)
	assert.Nil(t, account.RevokedAt)
	assert.True(t, len(account.ClientID) > len("agent-"))
	assert.Equal(t, "agent-", account.ClientID[:len("agent-")])
	assert.Len(t, secret, 43, "32 random bytes, base64url without padding")

	var hash string
	var public bool
	var grants []string
	require.NoError(t, db.QueryRow(ctx, `
		SELECT secret_hash, public, grant_types
		FROM auth.oauth2_clients WHERE id = $1`, account.ClientID,
	).Scan(&hash, &public, &grants))
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)))
	assert.NotContains(t, hash, secret)
	assert.False(t, public)
	assert.Equal(t, []string{"client_credentials"}, grants)

	var role, displayName string
	require.NoError(t, db.QueryRow(ctx, `
		SELECT role, COALESCE(display_name, '')
		FROM global.app_users WHERE id = $1`, account.UserID,
	).Scan(&role, &displayName))
	assert.Equal(t, "agent", role)
	assert.Equal(t, "Night shift", displayName)

	var passwordHash string
	require.NoError(t, db.QueryRow(ctx,
		`SELECT password_hash FROM auth.users WHERE id = $1`, account.UserID,
	).Scan(&passwordHash))
	assert.Equal(t, "!", passwordHash, "no password can match, so no sign-in")

	other, otherSecret, err := store.Create(ctx, "human:creator", "Night shift")
	require.NoError(t, err)
	assert.NotEqual(t, account.ClientID, other.ClientID)
	assert.NotEqual(t, account.UserID, other.UserID)
	assert.NotEqual(t, secret, otherSecret)
}

func TestListShowsNewestFirstIncludingRevoked(t *testing.T) {
	store, _ := newStore(t, nil)
	ctx := context.Background()
	first, _, err := store.Create(ctx, "human:a", "first")
	require.NoError(t, err)
	second, _, err := store.Create(ctx, "human:a", "second")
	require.NoError(t, err)
	_, err = store.Revoke(ctx, first.ID)
	require.NoError(t, err)

	accounts, err := store.List(ctx)
	require.NoError(t, err)

	var order []uuid.UUID
	var revokedFirst bool
	for _, a := range accounts {
		switch a.ID {
		case first.ID:
			order = append(order, a.ID)
			revokedFirst = a.RevokedAt != nil
		case second.ID:
			order = append(order, a.ID)
		}
	}
	assert.Equal(t, []uuid.UUID{second.ID, first.ID}, order)
	assert.True(t, revokedFirst)
}

func TestRevokeEndsTheClientOnceAndKeepsTheAccount(t *testing.T) {
	revoked := 0
	store, db := newStore(t, func() { revoked++ })
	ctx := context.Background()
	account, _, err := store.Create(ctx, "human:a", "to revoke")
	require.NoError(t, err)

	got, err := store.Revoke(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, got.RevokedAt)
	assert.Equal(t, 1, revoked)

	var clients int
	require.NoError(t, db.QueryRow(ctx,
		`SELECT count(*) FROM auth.oauth2_clients WHERE id = $1`, account.ClientID,
	).Scan(&clients))
	assert.Zero(t, clients)

	again, err := store.Revoke(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, got.RevokedAt, again.RevokedAt, "the first revoke time stands")

	_, err = store.Revoke(ctx, uuid.New())
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestClientSubjectIsTheUserOfAnActiveAccount(t *testing.T) {
	store, _ := newStore(t, nil)
	ctx := context.Background()
	account, _, err := store.Create(ctx, "human:a", "subject")
	require.NoError(t, err)

	subject, ok, err := store.ClientSubject(ctx, account.ClientID)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, account.UserID.String(), subject)

	_, ok, err = store.ClientSubject(ctx, "agent-unknown")
	require.NoError(t, err)
	assert.False(t, ok)

	_, err = store.Revoke(ctx, account.ID)
	require.NoError(t, err)
	_, ok, err = store.ClientSubject(ctx, account.ClientID)
	require.NoError(t, err)
	assert.False(t, ok, "a revoked account has no identity")
}

func TestAJobsAccountIsConfinedToItsTaskAndKeptOutOfTheList(t *testing.T) {
	store, _ := newStore(t, nil)
	ctx := context.Background()
	scope := agentaccounts.Scope{JobID: uuid.New(), TaskID: uuid.New()}

	account, secret, err := store.CreateForJob(
		ctx, "system:factory", "job #1 · claude", scope)
	require.NoError(t, err)
	require.NotNil(t, account.JobID)
	assert.Equal(t, scope.JobID, *account.JobID)
	assert.Equal(t, scope.TaskID, *account.TaskID)
	assert.NotEmpty(t, secret)

	got, err := store.ScopeOf(ctx, account.UserID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, scope, *got)

	// A person's account has no scope, and neither does an unknown user.
	plain, _, err := store.Create(ctx, "human:creator", "Night shift")
	require.NoError(t, err)
	none, err := store.ScopeOf(ctx, plain.UserID)
	require.NoError(t, err)
	assert.Nil(t, none)
	none, err = store.ScopeOf(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, none)

	// Job accounts stay out of the admin's list; the person's does not.
	list, err := store.List(ctx)
	require.NoError(t, err)
	ids := []uuid.UUID{}
	for _, a := range list {
		ids = append(ids, a.ID)
	}
	assert.Contains(t, ids, plain.ID)
	assert.NotContains(t, ids, account.ID)

	// It acts as an agent like any other account.
	subject, ok, err := store.ClientSubject(ctx, account.ClientID)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, account.UserID.String(), subject)
}

func TestARevokedJobAccountStopsWorkingButKeepsItsScope(t *testing.T) {
	var dropped int
	store, db := newStore(t, func() { dropped++ })
	ctx := context.Background()
	scope := agentaccounts.Scope{JobID: uuid.New(), TaskID: uuid.New()}
	account, _, err := store.CreateForJob(ctx, "system:factory", "job", scope)
	require.NoError(t, err)

	active, err := store.ActiveJobs(ctx)
	require.NoError(t, err)
	assert.Contains(t, active, scope.JobID)

	require.NoError(t, store.RevokeForJob(ctx, scope.JobID))
	assert.Equal(t, 1, dropped)
	_, ok, err := store.ClientSubject(ctx, account.ClientID)
	require.NoError(t, err)
	assert.False(t, ok)
	// Fail closed: a request already past authentication stays confined.
	got, err := store.ScopeOf(ctx, account.UserID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, scope, *got)
	active, err = store.ActiveJobs(ctx)
	require.NoError(t, err)
	assert.NotContains(t, active, scope.JobID)

	var kept int
	require.NoError(t, db.QueryRow(ctx,
		`SELECT count(*) FROM global.agent_accounts WHERE job_id = $1`, scope.JobID,
	).Scan(&kept))
	assert.Equal(t, 1, kept, "the record stays for history")

	// Revoking again, or for a job without an account, does nothing.
	require.NoError(t, store.RevokeForJob(ctx, scope.JobID))
	require.NoError(t, store.RevokeForJob(ctx, uuid.New()))
	assert.Equal(t, 1, dropped)
}

func TestAJobHasAtMostOneAccount(t *testing.T) {
	store, _ := newStore(t, nil)
	ctx := context.Background()
	scope := agentaccounts.Scope{JobID: uuid.New(), TaskID: uuid.New()}
	_, _, err := store.CreateForJob(ctx, "system:factory", "job", scope)
	require.NoError(t, err)
	_, _, err = store.CreateForJob(ctx, "system:factory", "job again", scope)
	require.Error(t, err)
}

func TestEveryAccountHoldsTheFactoryApp(t *testing.T) {
	store, db := newStore(t, nil)
	ctx := context.Background()

	standing, _, err := store.Create(ctx, "human:creator", "Standing")
	require.NoError(t, err)
	job, _, err := store.CreateForJob(ctx, "system", "Job", agentaccounts.Scope{
		JobID: uuid.New(), TaskID: uuid.New(),
	})
	require.NoError(t, err)

	access := func(userID uuid.UUID) []string {
		rows, qErr := db.Query(ctx,
			`SELECT app_name || ':' || level FROM global.app_access
			 WHERE user_id = $1`, userID.String())
		require.NoError(t, qErr)
		defer rows.Close()
		var apps []string
		for rows.Next() {
			var a string
			require.NoError(t, rows.Scan(&a))
			apps = append(apps, a)
		}
		return apps
	}
	assert.Equal(t, []string{"factory:write"}, access(standing.UserID))
	assert.Equal(t, []string{"factory:write"}, access(job.UserID))

	_, err = store.Revoke(ctx, standing.ID)
	require.NoError(t, err)
	assert.Empty(t, access(standing.UserID), "revoking drops the grants")
}

func TestRotateSecretIssuesANewSecretAndKeepsTheOldOneForADay(t *testing.T) {
	store, db := newStore(t, nil)
	ctx := context.Background()
	account, oldSecret, err := store.Create(ctx, "human:creator", "Rotating")
	require.NoError(t, err)

	rotated, newSecret, err := store.RotateSecret(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, account.ID, rotated.ID)
	assert.NotEqual(t, oldSecret, newSecret)

	var current, previous string
	var expiresIn float64
	require.NoError(t, db.QueryRow(ctx, `
		SELECT secret_hash, previous_secret_hash,
		       EXTRACT(EPOCH FROM previous_secret_expires_at - now())
		FROM auth.oauth2_clients WHERE id = $1`, account.ClientID,
	).Scan(&current, &previous, &expiresIn))
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(current), []byte(newSecret)))
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(previous), []byte(oldSecret)))
	assert.InDelta(t, agentaccounts.SecretOverlap.Seconds(), expiresIn, 60)
}

func TestRotateSecretRefusesRevokedJobAndUnknownAccounts(t *testing.T) {
	store, _ := newStore(t, nil)
	ctx := context.Background()

	revoked, _, err := store.Create(ctx, "human:creator", "Revoked")
	require.NoError(t, err)
	_, err = store.Revoke(ctx, revoked.ID)
	require.NoError(t, err)
	job, _, err := store.CreateForJob(ctx, "system", "Job", agentaccounts.Scope{
		JobID: uuid.New(), TaskID: uuid.New(),
	})
	require.NoError(t, err)

	for name, id := range map[string]uuid.UUID{"revoked": revoked.ID, "job": job.ID} {
		_, _, err = store.RotateSecret(ctx, id)
		require.ErrorIs(t, err, database.ErrResourceConflict, name)
	}
	_, _, err = store.RotateSecret(ctx, uuid.New())
	require.ErrorIs(t, err, database.ErrResourceNotFound)
}
