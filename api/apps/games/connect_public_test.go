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
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

// publicUserID owns the data behind the public-profile tests. It is distinct
// from userID so parallel test packages sharing the DB (cmd/api's
// ProfileService tests use userID) never fight over the same
// global.profile_shares row.
const publicUserID = "eeeeeeee-1111-2222-3333-444444444444"

const publicGamesToken = "test-games-profile-token"

// ensureProfileShare mirrors cmd/api/migrations/00006_profile_shares.sql so
// these tests can run before the cmd/api package has applied the global
// migrations, then links publicGamesToken to publicUserID.
func ensureProfileShare(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	stmts := []string{
		"CREATE SCHEMA IF NOT EXISTS global",
		`CREATE TABLE IF NOT EXISTS global.profile_shares (
			user_id TEXT PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	}
	for _, stmt := range stmts {
		_, err := testDB.Exec(ctx, stmt)
		require.NoError(t, err)
	}

	repo := sharedrepos.NewProfileSharesRepository(testDB)
	_, err := repo.Upsert(ctx, publicUserID, publicGamesToken)
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
