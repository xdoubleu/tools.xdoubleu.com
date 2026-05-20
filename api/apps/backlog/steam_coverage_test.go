package backlog_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

// seedSteamData imports steam games for userID using the mock client.
// It saves integrations with dummy steam credentials so ImportOwnedGames
// uses the mock factory (the actual keys are ignored by the mock).
//
//nolint:unused // May be useful for future tests.
func seedSteamData(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Save dummy integrations so ImportOwnedGames can build a client from
	// the factory.
	err := testApp.SaveIntegrations(
		ctx,
		userID,
		backlog.Integrations{ //nolint:exhaustruct //HardcoverAPIKey not needed for steam seed
			SteamAPIKey: "test-steam-api-key",
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	_, err = testApp.Services.Steam.ImportOwnedGames(ctx, userID)
	require.NoError(t, err)
}

// mockEmptyAchievementsSteamClient is a steam client whose GetPlayerAchievements
// returns an empty achievement list, triggering the UpsertAchievementSchemas path.
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

// TestSteamCompletionRate_NotFound covers the ErrResourceNotFound → "0.00"
// branch of GetCurrentSteamCompletionRate using an isolated user.
func TestSteamCompletionRate_NotFound(t *testing.T) {
	const isolatedUser = "steam-rate-notfound-user"
	rate, err := testApp.Services.Progress.GetCurrentSteamCompletionRate(
		context.Background(), isolatedUser,
	)
	require.NoError(t, err)
	assert.Equal(t, "0.00", rate)
}

// TestSteamCompletionRate_WithRecord saves a steam progress entry then reads it
// back, covering the return value branch of GetCurrentSteamCompletionRate.
func TestSteamCompletionRate_WithRecord(t *testing.T) {
	ctx := context.Background()
	const isolatedUser = "steam-rate-record-user"
	today := time.Now().UTC().Format(models.ProgressDateFormat)

	err := testApp.Services.Progress.Save(
		ctx, models.SteamTypeID, isolatedUser, []string{today}, []string{"55.00"},
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.progress WHERE user_id = $1 AND type_id = $2`,
			isolatedUser, models.SteamTypeID,
		)
	})

	rate, err := testApp.Services.Progress.GetCurrentSteamCompletionRate(
		ctx, isolatedUser,
	)
	require.NoError(t, err)
	assert.Equal(t, "55.00", rate)
}

// TestUpsertAchievementSchemas exercises the UpsertAchievementSchemas repository
// method by importing games with a steam client that returns no player achievements.
func TestUpsertAchievementSchemas(t *testing.T) {
	ctx := context.Background()
	const isolatedUserID = "upsert-schema-test-user-id"

	app2 := backlog.NewInner(
		ctx,
		sharedmocks.NewMockedAuthService(isolatedUserID),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory: func(_ string) steam.Client {
				return mockEmptyAchievementsSteamClient{}
			},
			HardcoverFactory: func(_ string) hardcover.Client {
				return nil
			},
		},
	)

	err := app2.SaveIntegrations(
		ctx,
		isolatedUserID,
		backlog.Integrations{ //nolint:exhaustruct //only steam needed
			SteamAPIKey: "test-key",
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	_, err = app2.Services.Steam.ImportOwnedGames(ctx, isolatedUserID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_games WHERE user_id = $1`,
			isolatedUserID,
		)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.integrations WHERE user_id = $1`,
			isolatedUserID,
		)
	})
}
