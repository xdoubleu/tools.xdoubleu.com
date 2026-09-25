package games_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/games/pkg/steam"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
	"tools.xdoubleu.com/internal/testhelper"
)

// twoAchievementsMock returns two achievements, only one with a global
// percent, to hit the sort comparator's nil branches.
type twoAchievementsMock struct{}

// fourAchievementsMock returns four achievements for game 9, two with and two
// without a global percent, covering every sort comparator branch.
type fourAchievementsMock struct{}

// TestConnectGetSteamGame_SortBranches covers GetSteamGame's nil-percent
// sort branches.
func TestConnectGetSteamGame_SortBranches(t *testing.T) {
	const isolatedUser = "sort-branch-test-user"

	app2 := games.NewInner(
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		func(_ string) steam.Client {
			return twoAchievementsMock{}
		},
	)

	err := app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		games.Integrations{
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	err = app2.Services.Steam.SyncUser(context.Background(), isolatedUser)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM games.steam_games WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM games.integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := gamesv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.GetSteamGameRequest{GameId: 7})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamGame(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Data)
	assert.Len(t, resp.Msg.Data.Achievements, 2)
}

// TestConnectRefreshSteamGame_SortBranches covers RefreshSteamGame's
// nil-percent sort branches and happy path.
func TestConnectRefreshSteamGame_SortBranches(t *testing.T) {
	const isolatedUser = "refresh-sort-branch-user"

	app2 := games.NewInner(
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		func(_ string) steam.Client { return twoAchievementsMock{} },
	)
	require.NoError(t, app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		games.Integrations{
			SteamUserID: "76561197960287930",
		},
	))
	require.NoError(t, app2.Services.Steam.SyncUser(context.Background(), isolatedUser))

	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM games.steam_achievements WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM games.steam_games WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM games.user_integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := gamesv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.RefreshSteamGameRequest{GameId: 7})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RefreshSteamGame(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Data)
	assert.Len(t, resp.Msg.Data.Achievements, 2)
}

// TestConnectRefreshSteamGame_AllSortBranches covers the remaining sort
// comparator branches.
func TestConnectRefreshSteamGame_AllSortBranches(t *testing.T) {
	const isolatedUser = "refresh-all-sort-user"
	ctx := context.Background()

	app2 := games.NewInner(
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		func(_ string) steam.Client { return fourAchievementsMock{} },
	)
	require.NoError(t, app2.SaveIntegrations(
		ctx,
		isolatedUser,
		games.Integrations{
			SteamUserID: "76561197960287930",
		},
	))
	require.NoError(t, app2.Services.Steam.SyncUser(ctx, isolatedUser))

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.steam_achievements WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.steam_games WHERE user_id = $1`, isolatedUser)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.user_integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := gamesv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req := connect.NewRequest(&gamesv1.RefreshSteamGameRequest{GameId: 9})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RefreshSteamGame(reqCtx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Data)
	assert.Len(t, resp.Msg.Data.Achievements, 4,
		"all four achievements should be returned after refresh")
}

// TestConnectRefreshSteamGame_SyncError: a SyncGame failure is CodeInternal.
func TestConnectRefreshSteamGame_SyncError(t *testing.T) {
	const isolatedUser = "refresh-sync-error-user"
	const gameID = 7

	app2 := games.NewInner(
		sharedmocks.NewMockedAuthService(isolatedUser),
		testApp.Logger,
		testCfg,
		testDB,
		func(_ string) steam.Client {
			// schemaErr makes SyncGame fail.
			return syncFakeClient{
				games:     []steam.Game{},
				playerAch: map[int][]steam.Achievement{},
				schemaErr: map[int]bool{gameID: true},
			}
		},
	)
	require.NoError(t, app2.SaveIntegrations(
		context.Background(),
		isolatedUser,
		games.Integrations{
			SteamUserID: "76561197960287930",
		},
	))

	t.Cleanup(func() {
		_, _ = testDB.Exec(context.Background(),
			`DELETE FROM games.user_integrations WHERE user_id = $1`, isolatedUser)
	})

	ts := httptest.NewServer(testhelper.BuildMux(app2))
	t.Cleanup(ts.Close)
	client := gamesv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&gamesv1.RefreshSteamGameRequest{GameId: int32(gameID)},
	)
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RefreshSteamGame(ctx, req)
	require.Error(t, err)
	var connectErr *connect.Error
	require.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

func (twoAchievementsMock) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only required fields
					AppID:                    7,
					Name:                     "two-ach game",
					HasCommunityVisibleStats: true,
				},
			},
		},
	}, nil
}

func (fourAchievementsMock) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only required fields
					AppID:                    9,
					Name:                     "four-ach game",
					HasCommunityVisibleStats: true,
				},
			},
		},
	}, nil
}

func (twoAchievementsMock) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:  true,
			SteamID:  steamID,
			GameName: "two-ach game",
			Achievements: []steam.Achievement{
				{
					APIName:     "ACH_A",
					Achieved:    1,
					UnlockTime:  0,
					Name:        "Alpha",
					Description: "",
				},
				{
					APIName:     "ACH_B",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "Beta",
					Description: "",
				},
			},
		},
	}, nil
}

func (fourAchievementsMock) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:  true,
			SteamID:  steamID,
			GameName: "four-ach game",
			Achievements: []steam.Achievement{
				// Two with global percents (different values → exercises *pi > *pj)
				{
					APIName:     "ACH_HIGH",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
				{
					APIName:     "ACH_LOW",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
				// Two without global percents (exercises both-nil DisplayName branch)
				{
					APIName:     "ACH_NIL_A",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
				{
					APIName:     "ACH_NIL_B",
					Achieved:    0,
					UnlockTime:  0,
					Name:        "",
					Description: "",
				},
			},
		},
	}, nil
}

func (twoAchievementsMock) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	//nolint:exhaustruct //skip
	return &steam.GetSchemaForGameResponse{}, nil
}

func (fourAchievementsMock) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	//nolint:exhaustruct //empty schema; DisplayNames default to ""
	return &steam.GetSchemaForGameResponse{}, nil
}

func (twoAchievementsMock) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	// Only ACH_A has a global percent.
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
		{Name: "ACH_A", Percent: "75.0"},
	}
	return &resp, nil
}

func (fourAchievementsMock) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
		{Name: "ACH_HIGH", Percent: "90.0"},
		{Name: "ACH_LOW", Percent: "10.0"},
		// ACH_NIL_A and ACH_NIL_B intentionally omitted → nil GlobalPercent
	}
	return &resp, nil
}
