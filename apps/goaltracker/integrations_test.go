package goaltracker_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tools.xdoubleu.com/apps/goaltracker"
)

func TestGetIntegrationsEmpty(t *testing.T) {
	// use a unique user with no data to guarantee empty result
	i, err := testApp.GetIntegrations(context.Background(), "no-integrations-user")
	require.NoError(t, err)
	assert.Empty(t, i.TodoistAPIKey)
	assert.Empty(t, i.SteamAPIKey)
	assert.Empty(t, i.GoodreadsURL)
}

func TestSaveAndGetIntegrations(t *testing.T) {
	ctx := context.Background()
	want := goaltracker.Integrations{
		TodoistAPIKey:    "key-123",
		TodoistProjectID: "proj-456",
		SteamAPIKey:      "steam-key",
		SteamUserID:      "steam-user",
		GoodreadsURL:     "https://goodreads.com/user/1",
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

	err := testApp.SaveIntegrations(ctx, userA, goaltracker.Integrations{
		TodoistAPIKey:    "user-a-key",
		TodoistProjectID: "",
		SteamAPIKey:      "",
		SteamUserID:      "",
		GoodreadsURL:     "",
	})
	require.NoError(t, err)

	err = testApp.SaveIntegrations(ctx, userB, goaltracker.Integrations{
		TodoistAPIKey:    "user-b-key",
		TodoistProjectID: "",
		SteamAPIKey:      "",
		SteamUserID:      "",
		GoodreadsURL:     "",
	})
	require.NoError(t, err)

	gotA, err := testApp.GetIntegrations(ctx, userA)
	require.NoError(t, err)
	assert.Equal(t, "user-a-key", gotA.TodoistAPIKey)

	gotB, err := testApp.GetIntegrations(ctx, userB)
	require.NoError(t, err)
	assert.Equal(t, "user-b-key", gotB.TodoistAPIKey)
}
