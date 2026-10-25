package learningpaths_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/crypto"
	sharedmodels "tools.xdoubleu.com/internal/models"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

// TestOAuthConnectionsRepository_RoundTrip covers Upsert, Get (decrypt),
// GetStatus, UpdateToken and Delete.
func TestOAuthConnectionsRepository_RoundTrip(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)
	repo := repositories.New(testDB, testSealer).OAuthConnections

	ctx := t.Context()
	user := "oauth-roundtrip-user"
	provider := sharedmodels.OAuthProviderTodoist

	//nolint:exhaustruct //other fields unused by this test
	tok := &oauth2.Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		Expiry:       time.Now().Add(time.Hour).Truncate(time.Second),
	}
	require.NoError(t, repo.Upsert(ctx, user, provider, tok, "data:read_write"))

	got, conn, err := repo.Get(ctx, user, provider)
	require.NoError(t, err)
	assert.Equal(t, "access-1", got.AccessToken)
	assert.Equal(t, "refresh-1", got.RefreshToken)
	assert.Equal(t, provider, conn.Provider)
	assert.Equal(t, user, conn.ConnectedBy)

	status, err := repo.GetStatus(ctx, user, provider)
	require.NoError(t, err)
	assert.False(t, status.ConnectedAt.IsZero())

	//nolint:exhaustruct //other fields unused by this test
	refreshed := &oauth2.Token{AccessToken: "access-2", RefreshToken: "refresh-1"}
	require.NoError(t, repo.UpdateToken(ctx, user, provider, refreshed))

	got, _, err = repo.Get(ctx, user, provider)
	require.NoError(t, err)
	assert.Equal(t, "access-2", got.AccessToken)

	require.NoError(t, repo.Delete(ctx, user, provider))
	_, _, err = repo.Get(ctx, user, provider)
	assert.Error(t, err)
}

// TestOAuthConnectionsRepository_EncryptionNotConfigured: without
// ENCRYPTION_KEY the sealer is nil, so token reads and writes must return
// ErrEncryptionNotConfigured instead of panicking.
func TestOAuthConnectionsRepository_EncryptionNotConfigured(t *testing.T) {
	testSealer, err := crypto.New(testCfg.EncryptionKey)
	require.NoError(t, err)

	ctx := t.Context()
	user := "oauth-no-key-user"
	provider := sharedmodels.OAuthProviderTodoist
	//nolint:exhaustruct //other fields unused by this test
	tok := &oauth2.Token{AccessToken: "access-1"}

	sealed := repositories.New(testDB, testSealer).OAuthConnections
	require.NoError(t, sealed.Upsert(ctx, user, provider, tok, ""))
	t.Cleanup(func() {
		_ = sealed.Delete(context.Background(), user, provider)
	})

	repo := repositories.New(testDB, nil).OAuthConnections

	err = repo.Upsert(ctx, user, provider, tok, "")
	require.ErrorIs(t, err, sharedrepos.ErrEncryptionNotConfigured)

	err = repo.UpdateToken(ctx, user, provider, tok)
	require.ErrorIs(t, err, sharedrepos.ErrEncryptionNotConfigured)

	_, _, err = repo.Get(ctx, user, provider)
	require.ErrorIs(t, err, sharedrepos.ErrEncryptionNotConfigured)
}

// TestOAuthConnectionsRepository_ReadsExistingCiphertext pins the on-disk
// format so stored rows keep decrypting: the hex values are raw
// Sealer.Encrypt output under this key, one per column.
func TestOAuthConnectionsRepository_ReadsExistingCiphertext(t *testing.T) {
	sealer, err := crypto.New("MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=")
	require.NoError(t, err)

	access, err := hex.DecodeString(
		"44289ace953da4664d1e006fd1f286d81eb938d1ea20162d01158cbe147d605c" +
			"47f2ab66cccdeb7cdb",
	)
	require.NoError(t, err)
	refresh, err := hex.DecodeString(
		"72db74d8b181876d6d3dcb1bcbd4a58297e3ac8095dcb324ff80f2127cfbb8d5" +
			"ae738aade499ba633453",
	)
	require.NoError(t, err)

	ctx := t.Context()
	user := "oauth-existing-ciphertext-user"
	provider := sharedmodels.OAuthProviderTodoist

	_, err = testDB.Exec(ctx, `
		INSERT INTO learningpaths.oauth_connections
			(user_id, provider, access_token, refresh_token)
		VALUES ($1, $2, $3, $4)
	`, user, provider, access, refresh)
	require.NoError(t, err)

	repo := repositories.New(testDB, sealer).OAuthConnections
	t.Cleanup(func() {
		_ = repo.Delete(context.Background(), user, provider)
	})

	got, _, err := repo.Get(ctx, user, provider)
	require.NoError(t, err)
	assert.Equal(t, "legacy-access", got.AccessToken)
	assert.Equal(t, "legacy-refresh", got.RefreshToken)
}
