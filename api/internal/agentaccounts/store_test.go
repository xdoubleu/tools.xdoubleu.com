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
