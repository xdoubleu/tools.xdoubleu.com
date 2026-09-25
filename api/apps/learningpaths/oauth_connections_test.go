package learningpaths_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/crypto"
	sharedmodels "tools.xdoubleu.com/internal/models"
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
	require.NoError(t, repo.Upsert(ctx, user, provider, tok))

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
