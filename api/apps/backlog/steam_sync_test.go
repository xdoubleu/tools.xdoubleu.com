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
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
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
			OpenLibrary:      nil,
			GoogleBooks:      nil,
			UniCat:           nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)

	err := app.SaveIntegrations(
		ctx,
		userID,
		backlog.Integrations{
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
			ImgIconURL:               "iconhasha",
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
	assert.Equal(
		t,
		games[0].GetFullImgIconURL(),
		gA.ImageURL,
		"game image url is derived from the Steam icon and persisted",
	)
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

// TestSyncUser_RefreshUpdatesListsAndRate reproduces the reported bug: after a
// refresh (second sync) the game lists and the dashboard "current rate" must
// reflect the freshly fetched achievements.
func TestSyncUser_RefreshUpdatesListsAndRate(t *testing.T) {
	ctx := context.Background()
	const user = "sync-refresh-user"
	const gameA = 8001

	games := []steam.Game{
		//nolint:exhaustruct //only required fields
		{AppID: gameA, Name: "Game A", HasCommunityVisibleStats: true},
	}

	// First sync: 1 of 4 achieved => 25.00
	app := newSyncTestApp(t, user, syncFakeClient{
		games: games,
		playerAch: map[int][]steam.Achievement{
			gameA: {ach("A1", 1), ach("A2", 0), ach("A3", 0), ach("A4", 0)},
		},
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app.Services.Steam.SyncUser(ctx, user))

	inProgress, err := app.Services.Steam.GetInProgress(ctx, user)
	require.NoError(t, err)
	require.Len(t, inProgress, 1)
	assert.Equal(t, "25.00", inProgress[0].CompletionRate)

	rate, err := app.Services.Progress.GetCurrentSteamCompletionRate(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, "25.00", rate)

	// Refresh: 2 of 4 achieved => 50.00
	app2 := newSyncTestApp(t, user, syncFakeClient{
		games: games,
		playerAch: map[int][]steam.Achievement{
			gameA: {ach("A1", 1), ach("A2", 1), ach("A3", 0), ach("A4", 0)},
		},
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app2.Services.Steam.SyncUser(ctx, user))

	inProgress, err = app2.Services.Steam.GetInProgress(ctx, user)
	require.NoError(t, err)
	require.Len(t, inProgress, 1)
	assert.Equal(t, "50.00", inProgress[0].CompletionRate,
		"refreshed game must show the updated completion rate")

	rate, err = app2.Services.Progress.GetCurrentSteamCompletionRate(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, "50.00", rate,
		"dashboard current rate must reflect the refreshed achievements")
}

// TestSyncUser_RatePreservedOnPartialFetchFailure reproduces the "current rate is
// wrong vs before" regression: on a refresh where one game's achievement fetch
// fails, the dashboard current rate must still reflect ALL games (the failed
// game keeps its persisted achievements), not just the games fetched this run.
func TestSyncUser_RatePreservedOnPartialFetchFailure(t *testing.T) {
	ctx := context.Background()
	const user = "sync-rate-partial-user"
	const gameA = 8101 // 1/4 => 25.00, always succeeds
	const gameB = 8102 // 4/4 => 100.00, fails on the refresh

	games := []steam.Game{
		//nolint:exhaustruct //only required fields
		{AppID: gameA, Name: "Game A", HasCommunityVisibleStats: true},
		//nolint:exhaustruct //only required fields
		{AppID: gameB, Name: "Game B", HasCommunityVisibleStats: true},
	}
	playerAch := map[int][]steam.Achievement{
		gameA: {ach("A1", 1), ach("A2", 0), ach("A3", 0), ach("A4", 0)},
		gameB: {ach("B1", 1), ach("B2", 1), ach("B3", 1), ach("B4", 1)},
	}

	// First sync: both succeed. Rate = avg(25, 100) = 62.50.
	app := newSyncTestApp(t, user, syncFakeClient{
		games: games, playerAch: playerAch, schemaErr: map[int]bool{},
	})
	require.NoError(t, app.Services.Steam.SyncUser(ctx, user))

	rate, err := app.Services.Progress.GetCurrentSteamCompletionRate(ctx, user)
	require.NoError(t, err)
	require.Equal(t, "62.50", rate)

	// Refresh: game B's achievement fetch fails. Its achievements are preserved,
	// so the rate must stay 62.50, not drop to 25.00.
	app2 := newSyncTestApp(t, user, syncFakeClient{
		games: games, playerAch: playerAch, schemaErr: map[int]bool{gameB: true},
	})
	require.NoError(t, app2.Services.Steam.SyncUser(ctx, user))

	rate, err = app2.Services.Progress.GetCurrentSteamCompletionRate(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, "62.50", rate,
		"current rate must include the game whose fetch failed (kept from DB)")
}

// TestSyncUser_RefetchesGameMetadata verifies that owned-game metadata (name and
// playtime) is refetched from Steam and persisted on every sync, not just on the
// first import.
func TestSyncUser_RefetchesGameMetadata(t *testing.T) {
	ctx := context.Background()
	const user = "sync-metadata-user"
	const gameID = 8201

	playerAch := map[int][]steam.Achievement{
		gameID: {ach("M1", 1), ach("M2", 0)},
	}

	app := newSyncTestApp(t, user, syncFakeClient{
		games: []steam.Game{
			//nolint:exhaustruct //only required fields
			{AppID: gameID, Name: "Old Name", PlaytimeForever: 60,
				HasCommunityVisibleStats: true},
		},
		playerAch: playerAch,
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app.Services.Steam.SyncUser(ctx, user))

	// Refresh: Steam now reports a new name and more playtime.
	app2 := newSyncTestApp(t, user, syncFakeClient{
		games: []steam.Game{
			//nolint:exhaustruct //only required fields
			{AppID: gameID, Name: "New Name", PlaytimeForever: 600,
				HasCommunityVisibleStats: true},
		},
		playerAch: playerAch,
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app2.Services.Steam.SyncUser(ctx, user))

	game, err := app2.Services.Steam.GetGameByID(ctx, gameID, user)
	require.NoError(t, err)
	assert.Equal(t, "New Name", game.Name, "game name must be refetched")
	assert.Equal(t, 600, game.Playtime, "game playtime must be refetched")
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

// TestSyncGame_UpdatesAchievements verifies that SyncGame refreshes the stored
// achievements and recomputes the game's completion rate for a single game
// without touching any other games.
func TestSyncGame_UpdatesAchievements(t *testing.T) {
	ctx := context.Background()
	const user = "syncgame-update-user"
	const gameID = 9001

	game := steam.Game{ //nolint:exhaustruct //only required fields
		AppID: gameID, Name: "SyncGame Test", HasCommunityVisibleStats: true,
	}

	// Seed: 1/4 achieved => 25.00
	app1 := newSyncTestApp(t, user, syncFakeClient{
		games: []steam.Game{game},
		playerAch: map[int][]steam.Achievement{
			gameID: {ach("G1", 1), ach("G2", 0), ach("G3", 0), ach("G4", 0)},
		},
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app1.Services.Steam.SyncUser(ctx, user))

	g, err := app1.Services.Steam.GetGameByID(ctx, gameID, user)
	require.NoError(t, err)
	assert.Equal(t, "25.00", g.CompletionRate)

	// SyncGame: 2/4 achieved => 50.00
	app2 := newSyncTestApp(t, user, syncFakeClient{
		games: []steam.Game{game},
		playerAch: map[int][]steam.Achievement{
			gameID: {ach("G1", 1), ach("G2", 1), ach("G3", 0), ach("G4", 0)},
		},
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app2.Services.Steam.SyncGame(ctx, user, gameID))

	g, err = app2.Services.Steam.GetGameByID(ctx, gameID, user)
	require.NoError(t, err)
	assert.Equal(t, "50.00", g.CompletionRate,
		"SyncGame must update the game's completion rate")
}

// TestSyncGame_UnconfiguredCreds verifies that SyncGame is a no-op when the
// user has no Steam credentials configured.
func TestSyncGame_UnconfiguredCreds(t *testing.T) {
	ctx := context.Background()
	const user = "syncgame-nocreds-user"

	// Create app without calling SaveIntegrations so creds are empty.
	app := backlog.NewInner(
		ctx,
		sharedmocks.NewMockedAuthService(user),
		testApp.Logger,
		testCfg,
		testDB,
		backlog.Clients{
			SteamFactory:     func(_ string) steam.Client { return nil },
			OpenLibrary:      nil,
			GoogleBooks:      nil,
			UniCat:           nil,
			ObjectStore:      objectstore.NewFake(),
			KoboStoreBaseURL: "",
			PublicAPIBaseURL: "",
		},
	)

	err := app.Services.Steam.SyncGame(ctx, user, 9999)
	assert.NoError(t, err, "SyncGame with no credentials must be a no-op")
}

// TestSyncGame_FetchErrorPropagates verifies that when the Steam API fetch
// fails, SyncGame returns the error and leaves the stored data unchanged.
func TestSyncGame_FetchErrorPropagates(t *testing.T) {
	ctx := context.Background()
	const user = "syncgame-error-user"
	const gameID = 9002

	game := steam.Game{ //nolint:exhaustruct //only required fields
		AppID: gameID, Name: "Error Game", HasCommunityVisibleStats: true,
	}

	// Seed: 1/2 achieved => 50.00
	app1 := newSyncTestApp(t, user, syncFakeClient{
		games: []steam.Game{game},
		playerAch: map[int][]steam.Achievement{
			gameID: {ach("E1", 1), ach("E2", 0)},
		},
		schemaErr: map[int]bool{},
	})
	require.NoError(t, app1.Services.Steam.SyncUser(ctx, user))

	g, err := app1.Services.Steam.GetGameByID(ctx, gameID, user)
	require.NoError(t, err)
	assert.Equal(t, "50.00", g.CompletionRate)

	// SyncGame with a schema fetch failure: must return error.
	app2 := newSyncTestApp(t, user, syncFakeClient{
		games:     []steam.Game{game},
		playerAch: map[int][]steam.Achievement{},
		schemaErr: map[int]bool{gameID: true},
	})
	err = app2.Services.Steam.SyncGame(ctx, user, gameID)
	assert.Error(t, err, "SyncGame must propagate Steam fetch errors")

	// Stored completion rate must be unchanged.
	g, err = app2.Services.Steam.GetGameByID(ctx, gameID, user)
	require.NoError(t, err)
	assert.Equal(t, "50.00", g.CompletionRate,
		"stored data must be unchanged after a failed SyncGame")
}
