package games_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/games/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

// seedSteamData imports steam games for userID via the mock client.
func seedSteamData(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	err := testApp.SaveIntegrations(
		ctx,
		userID,
		games.Integrations{
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	err = testApp.Services.Steam.SyncUser(ctx, userID)
	require.NoError(t, err)
}

// mockEmptyAchievementsSteamClient returns no player achievements, so the
// schema defines the set.
type mockEmptyAchievementsSteamClient struct{}

func (mockEmptyAchievementsSteamClient) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only required fields
					AppID:                    9999,
					Name:                     "no-achievements game",
					HasCommunityVisibleStats: true,
				},
			},
		},
	}, nil
}

func (mockEmptyAchievementsSteamClient) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:      true,
			SteamID:      steamID,
			GameName:     "no-achievements game",
			Achievements: []steam.Achievement{},
		},
	}, nil
}

func (mockEmptyAchievementsSteamClient) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	return &steam.GetSchemaForGameResponse{
		Game: steam.GameSchema{ //nolint:exhaustruct //only required fields
			AvailableGameStats: steam.AvailableGameStats{
				Achievements: []steam.AchievementSchema{
					{ //nolint:exhaustruct //only required fields
						Name:        "SCHEMA_ACH",
						DisplayName: "Schema Achievement",
					},
				},
			},
		},
	}, nil
}

func (mockEmptyAchievementsSteamClient) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{}
	return &resp, nil
}

// TestSteamCompletionRate_NotFound: no record yields "0.00".
func TestSteamCompletionRate_NotFound(t *testing.T) {
	const isolatedUser = "steam-rate-notfound-user"
	rate, err := testApp.Services.Progress.GetCurrentSteamCompletionRate(
		context.Background(), isolatedUser,
	)
	require.NoError(t, err)
	assert.Equal(t, "0.00", rate)
}

// TestSteamCompletionRate_WithRecord: a saved rate is read back.
func TestSteamCompletionRate_WithRecord(t *testing.T) {
	ctx := context.Background()
	const isolatedUser = "steam-rate-record-user"
	today := time.Now().UTC().Format(models.ProgressDateFormat)

	err := testApp.Services.Progress.Save(
		ctx, isolatedUser, []string{today}, []string{"55.00"},
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.progress WHERE user_id = $1`,
			isolatedUser,
		)
	})

	rate, err := testApp.Services.Progress.GetCurrentSteamCompletionRate(
		ctx, isolatedUser,
	)
	require.NoError(t, err)
	assert.Equal(t, "55.00", rate)
}

// TestGetRecentlyActiveGames_Repo: played games are returned by last_played;
// never-played games are excluded.
func TestGetRecentlyActiveGames_Repo(t *testing.T) {
	seedSteamData(t)
	ctx := context.Background()

	games, err := testApp.Repositories.Steam.GetRecentlyActiveGames(
		ctx, userID, 10,
	)
	require.NoError(t, err)

	var found *models.RecentGame
	for i := range games {
		if games[i].ID == 1 {
			found = &games[i]
		}
	}
	require.NotNil(t, found, "seeded game should be recently active")
	assert.False(t, found.LastPlayed.IsZero())

	// A game with unlocks but no last_played drops out.
	_, err = testDB.Exec(
		ctx,
		"UPDATE games.steam_games SET last_played = NULL WHERE id = $1 AND user_id = $2",
		1,
		userID,
	)
	require.NoError(t, err)

	afterClear, err := testApp.Repositories.Steam.GetRecentlyActiveGames(
		ctx, userID, 10,
	)
	require.NoError(t, err)
	for _, g := range afterClear {
		assert.NotEqual(t, 1, g.ID, "game with no last_played should be excluded")
	}
}

// TestGetRecentlyActive_Service covers the service wrapper.
func TestGetRecentlyActive_Service(t *testing.T) {
	seedSteamData(t)

	games, err := testApp.Services.Steam.GetRecentlyActive(
		context.Background(), userID,
	)
	require.NoError(t, err)

	found := false
	for _, g := range games {
		if g.ID == 1 {
			found = true
		}
	}
	assert.True(t, found, "seeded game should be returned by the service")
}

// TestUpsertGames_SetsLastSyncedAt: last_synced_at is written and read back.
func TestUpsertGames_SetsLastSyncedAt(t *testing.T) {
	ctx := context.Background()
	const isolatedUser = "last-synced-at-test-user"

	game := &models.Game{ //nolint:exhaustruct //defaults are fine for test
		ID:             88881,
		Name:           "sync-ts test game",
		IsDelisted:     false,
		CompletionRate: "0.00",
		Contribution:   "0.0000",
		Playtime:       0,
		ImageURL:       "",
	}

	err := testApp.Repositories.Steam.UpsertGames(ctx, nil, map[int]*models.Game{
		game.ID: game,
	}, isolatedUser)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.steam_games WHERE user_id = $1`,
			isolatedUser,
		)
	})

	got, err := testApp.Repositories.Steam.GetGameByID(ctx, game.ID, isolatedUser)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.LastSyncedAt.IsZero(),
		"UpsertGames should set last_synced_at to a non-zero timestamp")
}

// TestSyncUser_SchemaOnlyAchievements covers the schema-only path.
func TestSyncUser_SchemaOnlyAchievements(t *testing.T) {
	ctx := context.Background()
	const isolatedUserID = "upsert-schema-test-user-id"

	app2 := games.NewInner(
		sharedmocks.NewMockedAuthService(isolatedUserID),
		testApp.Logger,
		testCfg,
		testDB,
		func(_ string) steam.Client {
			return mockEmptyAchievementsSteamClient{}
		},
	)

	err := app2.SaveIntegrations(
		ctx,
		isolatedUserID,
		games.Integrations{
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	err = app2.Services.Steam.SyncUser(ctx, isolatedUserID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.steam_games WHERE user_id = $1`,
			isolatedUserID,
		)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.integrations WHERE user_id = $1`,
			isolatedUserID,
		)
	})
}
