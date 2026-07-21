package repositories_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/repositories"
)

func clearOAuthConnections(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(t.Context(), "DELETE FROM global.oauth_connections")
	require.NoError(t, err)
}

func testSealer(t *testing.T) *crypto.Sealer {
	t.Helper()
	sealer, err := crypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)
	return sealer
}

func TestOAuthConnectionsRoundTrip(t *testing.T) {
	clearOAuthConnections(t)
	repo := repositories.NewOAuthConnectionsRepository(testDB, testSealer(t))

	_, _, err := repo.Get(t.Context(), models.OAuthProviderGithub)
	assert.ErrorIs(
		t,
		err,
		database.ErrResourceNotFound,
		"no connection should exist yet",
	)

	expiry := time.Now().Add(time.Hour).Truncate(time.Second).UTC()
	require.NoError(
		t,
		repo.Upsert(
			t.Context(),
			models.OAuthProviderGithub,
			&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
				AccessToken:  "access-1",
				RefreshToken: "refresh-1",
				Expiry:       expiry,
			},
			"admin-user",
		),
	)

	tok, conn, err := repo.Get(t.Context(), models.OAuthProviderGithub)
	require.NoError(t, err)
	assert.Equal(t, "access-1", tok.AccessToken)
	assert.Equal(t, "refresh-1", tok.RefreshToken)
	assert.True(t, expiry.Equal(tok.Expiry))
	assert.Equal(t, "admin-user", conn.ConnectedBy)
	require.NotNil(t, conn.ExpiresAt)
	assert.True(t, expiry.Equal(*conn.ExpiresAt))

	// UpdateToken rotates the token but keeps connected_by/connected_at.
	require.NoError(
		t,
		repo.UpdateToken(
			t.Context(),
			models.OAuthProviderGithub,
			&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
				AccessToken:  "access-2",
				RefreshToken: "refresh-1",
			},
		),
	)
	tok, conn, err = repo.Get(t.Context(), models.OAuthProviderGithub)
	require.NoError(t, err)
	assert.Equal(t, "access-2", tok.AccessToken)
	assert.Equal(t, "admin-user", conn.ConnectedBy)

	list, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, models.OAuthProviderGithub, list[0].Provider)

	require.NoError(t, repo.Delete(t.Context(), models.OAuthProviderGithub))
	_, _, err = repo.Get(t.Context(), models.OAuthProviderGithub)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)
}

func TestOAuthConnectionsIndependentPerProvider(t *testing.T) {
	clearOAuthConnections(t)
	repo := repositories.NewOAuthConnectionsRepository(testDB, testSealer(t))

	require.NoError(t, repo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "gh-token",
		},
		"admin",
	))
	require.NoError(t, repo.Upsert(
		t.Context(),
		models.OAuthProviderSentry,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "sentry-token",
		},
		"admin",
	))

	require.NoError(t, repo.Delete(t.Context(), models.OAuthProviderGithub))

	_, _, err := repo.Get(t.Context(), models.OAuthProviderGithub)
	assert.ErrorIs(t, err, database.ErrResourceNotFound)

	tok, _, err := repo.Get(t.Context(), models.OAuthProviderSentry)
	require.NoError(t, err)
	assert.Equal(t, "sentry-token", tok.AccessToken)
}

func TestOAuthConnectionsSetConfig(t *testing.T) {
	clearOAuthConnections(t)
	repo := repositories.NewOAuthConnectionsRepository(testDB, testSealer(t))

	err := repo.SetConfig(
		t.Context(), models.OAuthProviderGithub, []byte(`{"repo":"o/r"}`),
	)
	assert.ErrorIs(
		t, err, database.ErrResourceNotFound,
		"configuring an unconnected provider must not silently no-op",
	)

	require.NoError(t, repo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other token fields unused in test
			AccessToken: "gh-token",
		},
		"admin",
	))

	_, conn, err := repo.Get(t.Context(), models.OAuthProviderGithub)
	require.NoError(t, err)
	assert.Empty(t, conn.Config, "config is nil until explicitly set")

	require.NoError(t, repo.SetConfig(
		t.Context(), models.OAuthProviderGithub, []byte(`{"repo":"o/r"}`),
	))

	_, conn, err = repo.Get(t.Context(), models.OAuthProviderGithub)
	require.NoError(t, err)
	assert.JSONEq(t, `{"repo":"o/r"}`, string(conn.Config))

	list, err := repo.List(t.Context())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.JSONEq(t, `{"repo":"o/r"}`, string(list[0].Config))
}

func TestOAuthConnectionsSetConfigRejectsInvalidJSON(t *testing.T) {
	clearOAuthConnections(t)
	repo := repositories.NewOAuthConnectionsRepository(testDB, testSealer(t))

	require.NoError(t, repo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "gh-token",
		},
		"admin",
	))

	// Postgres would otherwise reject this with a raw SQLSTATE 22P02
	// (invalid input syntax for type json) surfacing as an unscrubbed
	// CodeInternal — SetConfig must catch it before it reaches the DB.
	err := repo.SetConfig(
		t.Context(), models.OAuthProviderGithub, []byte("not json"),
	)
	assert.ErrorIs(t, err, repositories.ErrInvalidConfig)

	err = repo.SetConfig(
		t.Context(), models.OAuthProviderGithub, []byte{},
	)
	assert.ErrorIs(t, err, repositories.ErrInvalidConfig)
}

func TestOAuthConnectionsRepository_EncryptionNotConfigured(t *testing.T) {
	clearOAuthConnections(t)
	repo := repositories.NewOAuthConnectionsRepository(testDB, nil)

	err := repo.Upsert(
		t.Context(),
		models.OAuthProviderGithub,
		&oauth2.Token{ //nolint:exhaustruct // other fields unused in test
			AccessToken: "x",
		},
		"admin",
	)
	assert.ErrorIs(t, err, repositories.ErrEncryptionNotConfigured)
}
