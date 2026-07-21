package oauthconn_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/oauthconn"
)

func TestStateStore_RoundTrip(t *testing.T) {
	store := oauthconn.NewStateStore()
	state := store.New(models.OAuthProviderGithub, "user-1")

	provider, userID, ok := store.Consume(state)
	require.True(t, ok)
	assert.Equal(t, models.OAuthProviderGithub, provider)
	assert.Equal(t, "user-1", userID)
}

func TestStateStore_SingleUse(t *testing.T) {
	store := oauthconn.NewStateStore()
	state := store.New(models.OAuthProviderGithub, "user-1")

	_, _, ok := store.Consume(state)
	require.True(t, ok)

	_, _, ok = store.Consume(state)
	assert.False(t, ok, "a state token must not be usable twice")
}

func TestStateStore_UnknownState(t *testing.T) {
	store := oauthconn.NewStateStore()
	_, _, ok := store.Consume("does-not-exist")
	assert.False(t, ok)
}
