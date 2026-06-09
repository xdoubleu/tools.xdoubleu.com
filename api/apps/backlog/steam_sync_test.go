package backlog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	sharedmocks "tools.xdoubleu.com/internal/mocks"
)

// syncFakeClient is a configurable Steam client for SyncUser tests. It serves a
// fixed set of owned games and per-game player achievements, and can be told to
// fail GetSchemaForGame for specific app IDs to simulate a transient per-game
// fetch failure.
type syncFakeClient struct {
	games     []steam.Game
	playerAch map[int][]steam.Achievement
	schemaErr map[int]bool
}

func (c syncFakeClient) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	return &steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: len(c.games),
			Games:     c.games,
		},
	}, nil
}

func (c syncFakeClient) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	appID int,
) (*steam.AchievementsResponse, error) {
	return &steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:      true,
			SteamID:      steamID,
			GameName:     "",
			Achievements: c.playerAch[appID],
		},
	}, nil
}

func (c syncFakeClient) GetSchemaForGame(
	_ context.Context,
	appID int,
) (*steam.GetSchemaForGameResponse, error) {
	if c.schemaErr[appID] {
		return nil, errors.New("schema unavailable")
	}
	//nolint:exhaustruct //empty schema is enough; completion uses player counts
	return &steam.GetSchemaForGameResponse{}, nil
}

func (c syncFakeClient) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{}
	return &resp, nil
}

func newSyncTestApp(
	t *testing.T,
	userID string,
	client steam.Client,
) *backlog.Backlog {
	t.Helper()
	ctx := context.Background()

	app := backlog.NewInner(
		ctx,
		sharedmocks.NewMockedAuthService(userID),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory:     func(_ string) steam.Client { return client },
			HardcoverFactory: func(_ string) hardcover.Client { return nil },
		},
	)

	err := app.SaveIntegrations(
		ctx,
		userID,
		backlog.Integrations{ //nolint:exhaustruct //only steam
			SteamAPIKey: "test-key",
			SteamUserID: "76561197960287930",
		},
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_achievements WHERE user_id = $1`, userID)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_games WHERE user_id = $1`, userID)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.progress WHERE user_id = $1`, userID)
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.user_integrations WHERE user_id = $1`, userID)
	})

	return app
}

func ach(apiName string, achieved int) steam.Achievement {
	return steam.Achievement{
		APIName:     apiName,
		Achieved:    achieved,
		UnlockTime:  1700000000,
		Name:        apiName,
		Description: "",
	}
}

// TestSyncUser_SchemaFailurePreservesPriorCompletion is the regression test for
// the corruption bug: when GetSchemaForGame fails for one game during a refresh,
// that game must keep its previously computed completion_rate (not be reset to
// "0.00"), the other games must still be refreshed, and the sync must commit.
func TestSyncUser_SchemaFailurePreservesPriorCompletion(t *testing.T) {
	ctx := context.Background()
	const user = "sync-preserve-user"
	const gameA = 5001 // refreshed successfully on both runs
	const gameB = 5002 // schema fetch fails on the second run

	games := []steam.Game{
		//nolint:exhaustruct //only required fields
		{
			AppID:                    gameA,
			Name:                     "Game A",
			HasCommunityVisibleStats: true,
		},
		//nolint:exhaustruct //only required fields
		{
			AppID:                    gameB,
			Name:                     "Game B",
			HasCommunityVisibleStats: true,
		},
	}
	playerAch := map[int][]steam.Achievement{
		gameA: {ach("A1", 1), ach("A2", 0)}, // 1/2 => 50.00
		gameB: {ach("B1", 1), ach("B2", 1)}, // 2/2 => 100.00
	}

	// First run: everything succeeds.
	app := newSyncTestApp(t, user, syncFakeClient{
		games:     games,
		playerAch: playerAch,
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app.Services.Steam.SyncUser(ctx, user))

	gA, err := app.Services.Steam.GetGameByID(ctx, gameA, user)
	require.NoError(t, err)
	assert.Equal(t, "50.00", gA.CompletionRate)
	gB, err := app.Services.Steam.GetGameByID(ctx, gameB, user)
	require.NoError(t, err)
	assert.Equal(t, "100.00", gB.CompletionRate)

	// Second run: schema fetch fails for game B only.
	app2 := newSyncTestApp(t, user, syncFakeClient{
		games:     games,
		playerAch: playerAch,
		schemaErr: map[int]bool{gameB: true},
	})
	require.NoError(t, app2.Services.Steam.SyncUser(ctx, user))

	gA, err = app2.Services.Steam.GetGameByID(ctx, gameA, user)
	require.NoError(t, err)
	assert.Equal(
		t,
		"50.00",
		gA.CompletionRate,
		"successfully fetched game stays correct",
	)

	gB, err = app2.Services.Steam.GetGameByID(ctx, gameB, user)
	require.NoError(t, err)
	assert.Equal(t, "100.00", gB.CompletionRate,
		"game whose fetch failed keeps its prior completion rate, not 0.00")
}

// TestSyncUser_NoLongerOwnedGameIsDelisted verifies that a game that drops out of
// the owned list on a later sync is carried over and marked delisted (keeping its
// stored completion rate) rather than removed.
func TestSyncUser_NoLongerOwnedGameIsDelisted(t *testing.T) {
	ctx := context.Background()
	const user = "sync-delist-user"
	const keptGame = 7001
	const droppedGame = 7002

	bothGames := []steam.Game{
		//nolint:exhaustruct //only required fields
		{AppID: keptGame, Name: "Kept", HasCommunityVisibleStats: true},
		//nolint:exhaustruct //only required fields
		{AppID: droppedGame, Name: "Dropped", HasCommunityVisibleStats: true},
	}
	playerAch := map[int][]steam.Achievement{
		keptGame:    {ach("K1", 1), ach("K2", 0)},
		droppedGame: {ach("D1", 1), ach("D2", 1)},
	}

	app := newSyncTestApp(t, user, syncFakeClient{
		games:     bothGames,
		playerAch: playerAch,
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app.Services.Steam.SyncUser(ctx, user))

	// Re-sync with the dropped game no longer owned.
	app2 := newSyncTestApp(t, user, syncFakeClient{
		games:     bothGames[:1],
		playerAch: playerAch,
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app2.Services.Steam.SyncUser(ctx, user))

	dropped, err := app2.Services.Steam.GetGameByID(ctx, droppedGame, user)
	require.NoError(t, err)
	assert.True(t, dropped.IsDelisted, "no-longer-owned game is marked delisted")
	assert.Equal(t, "100.00", dropped.CompletionRate, "delisted game keeps its rate")
}

// TestSteamWithTx_CommitAndRollback verifies the transaction wrapper: a fn that
// returns nil commits its writes, and a fn that returns an error rolls them back
// so the database is left unchanged.
func TestSteamWithTx_CommitAndRollback(t *testing.T) {
	ctx := context.Background()
	repo := testApp.Repositories.Steam
	const user = "withtx-user"
	const committedID = 6001
	const rolledBackID = 6002

	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM backlog.steam_games WHERE user_id = $1`, user)
	})

	game := func(id int) map[int]*models.Game {
		return map[int]*models.Game{
			id: {
				ID: id, Name: "tx game", IsDelisted: false,
				CompletionRate: "0.00", Contribution: "0.0000", Playtime: 0,
			},
		}
	}

	// Commit path.
	err := repo.WithTx(ctx, func(tx pgx.Tx) error {
		return repo.UpsertGames(ctx, tx, game(committedID), user)
	})
	require.NoError(t, err)

	stored, err := repo.GetGameByID(ctx, committedID, user)
	require.NoError(t, err)
	assert.Equal(t, committedID, stored.ID)

	// Rollback path: write a row, then return an error from the callback.
	sentinel := errors.New("boom")
	err = repo.WithTx(ctx, func(tx pgx.Tx) error {
		if errIn := repo.UpsertGames(ctx, tx, game(rolledBackID), user); errIn != nil {
			return errIn
		}
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	_, err = repo.GetGameByID(ctx, rolledBackID, user)
	require.Error(t, err, "rolled-back game must not be persisted")
}
