package games_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games"
)

func TestGetIntegrationsEmpty(t *testing.T) {
	i, err := testApp.GetIntegrations(context.Background(), "no-integrations-user")
	require.NoError(t, err)
	assert.Empty(t, i.SteamUserID)
}

func TestSaveAndGetIntegrations(t *testing.T) {
	ctx := context.Background()
	want := games.Integrations{
		SteamUserID: "steam-user",
	}

	err := testApp.SaveIntegrations(ctx, userID, want)
	require.NoError(t, err)

	got, err := testApp.GetIntegrations(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSaveIntegrationsIsolation(t *testing.T) {
	ctx := context.Background()
	userA := "isolation-user-a"
	userB := "isolation-user-b"

	err := testApp.SaveIntegrations(ctx, userA, games.Integrations{
		SteamUserID: "111111",
	})
	require.NoError(t, err)

	err = testApp.SaveIntegrations(ctx, userB, games.Integrations{
		SteamUserID: "222222",
	})
	require.NoError(t, err)

	gotA, err := testApp.GetIntegrations(ctx, userA)
	require.NoError(t, err)
	assert.Equal(t, "111111", gotA.SteamUserID)

	gotB, err := testApp.GetIntegrations(ctx, userB)
	require.NoError(t, err)
	assert.Equal(t, "222222", gotB.SteamUserID)
}
