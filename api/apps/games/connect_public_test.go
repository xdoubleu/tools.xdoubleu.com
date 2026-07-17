package games_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	sharedmodels "tools.xdoubleu.com/internal/models"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

// publicUserID owns the data behind the public-profile tests. It is distinct
// from userID so parallel test packages sharing the DB (cmd/api's
// ProfileService tests use userID) never fight over the same
// global.profile_shares row.
const publicUserID = "eeeeeeee-1111-2222-3333-444444444444"

const publicGamesToken = "test-games-profile-token"
const publicDisplayName = "Public Games Owner"

// ensureProfileShare mirrors cmd/api/migrations/00001_init.sql and
// 00007_profile_shares_per_app.sql so these tests can run before the
// cmd/api package has applied the global migrations, then links
// publicGamesToken to publicUserID with a display name.
func ensureProfileShare(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	stmts := []string{
		"CREATE SCHEMA IF NOT EXISTS global",
		`CREATE TABLE IF NOT EXISTS global.app_users (
			id           TEXT PRIMARY KEY,
			email        TEXT NOT NULL,
			last_seen    TIMESTAMPTZ NOT NULL DEFAULT now(),
			role         TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			display_name TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS global.profile_shares (
			user_id TEXT NOT NULL,
			app TEXT NOT NULL CHECK (app IN ('books', 'games')),
			token TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (user_id, app)
		)`,
	}
	for _, stmt := range stmts {
		_, err := testDB.Exec(ctx, stmt)
		require.NoError(t, err)
	}

	_, err := testDB.Exec(ctx, `
		INSERT INTO global.app_users (id, email, display_name)
		VALUES ($1, 'public-owner@example.com', $2)
		ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name
	`, publicUserID, publicDisplayName)
	require.NoError(t, err)

	repo := sharedrepos.NewProfileSharesRepository(testDB)
	_, err = repo.Upsert(
		ctx,
		publicUserID,
		sharedmodels.ProfileAppGames,
		publicGamesToken,
	)
	require.NoError(t, err)
}

// seedPublicSteamData syncs the mocked Steam library for publicUserID.
func seedPublicSteamData(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	err := testApp.SaveIntegrations(ctx, publicUserID, games.Integrations{
		SteamUserID: "76561197960287930",
	})
	require.NoError(t, err)

	require.NoError(t, testApp.Services.Steam.SyncUser(ctx, publicUserID))
}

// newPublicGamesClient returns a client with NO auth cookie — the public
// service must work without a session.
func newPublicGamesClient(t *testing.T) gamesv1connect.PublicGamesServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return gamesv1connect.NewPublicGamesServiceClient(http.DefaultClient, ts.URL)
}

func TestGetSharedSteam_Success(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicGamesClient(t)
	resp, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedSteamRequest{
			Token: publicGamesToken,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Steam)
	assert.NotEmpty(t, resp.Msg.LastSyncedAt,
		"synced library should report a last_synced_at")
	assert.Equal(t, publicDisplayName, resp.Msg.DisplayName)
	total := len(resp.Msg.Steam.NotStarted) +
		len(resp.Msg.Steam.InProgress) +
		len(resp.Msg.Steam.Completed)
	assert.Positive(t, total, "seeded library should contain games")
}

func TestGetSharedSteam_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicGamesClient(t)
	_, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedSteamRequest{
			Token: "definitely-not-a-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteam_EmptyToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicGamesClient(t)
	_, err := client.GetSharedSteam(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedSteamRequest{}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteam_WrongAppToken(t *testing.T) {
	ensureProfileShare(t)
	ctx := context.Background()

	// A token minted for the books app must not resolve on the games
	// endpoint, even though it belongs to the same user.
	repo := sharedrepos.NewProfileSharesRepository(testDB)
	_, err := repo.Upsert(
		ctx,
		publicUserID,
		sharedmodels.ProfileAppReading,
		"cross-app-books-token",
	)
	require.NoError(t, err)

	client := newPublicGamesClient(t)
	_, err = client.GetSharedSteam(
		ctx,
		connect.NewRequest(&gamesv1.GetSharedSteamRequest{
			Token: "cross-app-books-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteamGame_Success(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicGamesClient(t)
	resp, err := client.GetSharedSteamGame(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedSteamGameRequest{
			Token:  publicGamesToken,
			GameId: 1,
		}),
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Data)
	require.NotNil(t, resp.Msg.Data.Game)
	assert.NotEmpty(t, resp.Msg.Data.Game.LastSyncedAt)
}

func TestGetSharedSteamGame_UnknownGame(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicGamesClient(t)
	_, err := client.GetSharedSteamGame(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedSteamGameRequest{
			Token:  publicGamesToken,
			GameId: 999999,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedSteamGame_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicGamesClient(t)
	_, err := client.GetSharedSteamGame(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedSteamGameRequest{
			Token:  "definitely-not-a-token",
			GameId: 1,
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedRecentlyActiveGames_UnknownToken(t *testing.T) {
	ensureProfileShare(t)

	client := newPublicGamesClient(t)
	_, err := client.GetSharedRecentlyActiveGames(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedRecentlyActiveGamesRequest{
			Token: "definitely-not-a-token",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestGetSharedRecentlyActiveGames_Success(t *testing.T) {
	ensureProfileShare(t)
	seedPublicSteamData(t)

	client := newPublicGamesClient(t)
	resp, err := client.GetSharedRecentlyActiveGames(
		context.Background(),
		connect.NewRequest(&gamesv1.GetSharedRecentlyActiveGamesRequest{
			Token: publicGamesToken,
		}),
	)
	require.NoError(t, err)
	assert.NotNil(t, resp.Msg.Games)
}
