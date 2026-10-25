package games_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/games/pkg/steam"
	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
	"tools.xdoubleu.com/internal/testhelper"
)

func newGamesTestClient(t *testing.T) gamesv1connect.GamesServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return gamesv1connect.NewGamesServiceClient(connect.NewClient(connecthttp.NewTransport(
		http.DefaultClient,
		ts.URL,
		connecthttp.WithReadMaxBytes(0),
		connecthttp.WithHTTPGet(),
	)))
}

func TestConnectGetSteam(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &gamesv1.GetSteamRequest{}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetSteam(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp.Steam != nil {
		assert.GreaterOrEqual(t, int(resp.Steam.TotalBacklog), 0)
		assert.NotNil(t, resp.Steam.Distribution)
	}
}

func TestConnectGetSteamGame(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &gamesv1.GetSteamGameRequest{
		GameId: 570,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	// Only checks the call doesn't panic; the mock may lack the game.
	_, _ = client.GetSteamGame(ctx, req)
}

func TestConnectGetSteamGame_WithSeededData(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &gamesv1.GetSteamGameRequest{
		GameId: 1,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamGame(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Data)
	assert.NotNil(t, resp.Data.Game)
	assert.NotEmpty(t, resp.Data.Game.LastSyncedAt,
		"GetSteamGame should populate last_synced_at")
}

func TestConnectGetRecentlyActiveGames(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &gamesv1.GetRecentlyActiveGamesRequest{}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetRecentlyActiveGames(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Games)

	found := false
	for _, g := range resp.Games {
		if g.Id == 1 {
			found = true
			assert.NotEmpty(t, g.LastPlayedAt)
		}
	}
	assert.True(t, found, "seeded game should appear in recent activity")
}

func TestConnectGetSteamDistribution_ValidBucket(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &gamesv1.GetSteamDistributionRequest{
		Bucket: 0,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamDistribution(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp.Data != nil {
		assert.Equal(t, "0–9%", resp.Data.Label)
	}
}

func TestConnectGetSteamDistribution_InvalidBucket(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &gamesv1.GetSteamDistributionRequest{
		Bucket: 99,
	}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.GetSteamDistribution(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

func TestConnectGetSteamGame_UnknownGameIsNotFound(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, info := connect.NewClientContext(t.Context())
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.GetSteamGame(ctx, &gamesv1.GetSteamGameRequest{GameId: 999999})
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestConnectRefreshSteamGame_GameNotFound: an unknown game is NotFound
// without Steam configured.
func TestConnectRefreshSteamGame_GameNotFound(t *testing.T) {
	//nolint:exhaustruct //empty: Steam not configured
	require.NoError(t, testApp.SaveIntegrations(
		t.Context(), userID, games.Integrations{},
	))
	client := newGamesTestClient(t)
	ctx, info := connect.NewClientContext(t.Context())
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.RefreshSteamGame(
		ctx,
		&gamesv1.RefreshSteamGameRequest{GameId: 999999},
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// TestConnectRefreshSteamGame_UnknownGameSkipsSteam: with Steam configured, an
// unknown game is NotFound before Steam (which rejects unknown apps) is asked.
func TestConnectRefreshSteamGame_UnknownGameSkipsSteam(t *testing.T) {
	const unknownGame = 999999
	app := newSyncTestApp(t, "refresh-unknown-game-user", syncFakeClient{
		games:     []steam.Game{},
		playerAch: map[int][]steam.Achievement{},
		schemaErr: map[int]bool{unknownGame: true},
	})
	ts := httptest.NewServer(testhelper.BuildMux(app))
	t.Cleanup(ts.Close)
	client := gamesv1connect.NewGamesServiceClient(connect.NewClient(
		connecthttp.NewTransport(http.DefaultClient, ts.URL, connecthttp.WithHTTPGet()),
	))
	ctx, info := connect.NewClientContext(t.Context())
	info.RequestHeader().Set("Cookie", accessToken.String())

	_, err := client.RefreshSteamGame(
		ctx,
		&gamesv1.RefreshSteamGameRequest{GameId: unknownGame},
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConnectRefreshSteamGame_WithSeededData(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &gamesv1.RefreshSteamGameRequest{GameId: 1}
	ctx, info := connect.NewClientContext(ctx)
	info.RequestHeader().Set("Cookie", accessToken.String())

	resp, err := client.RefreshSteamGame(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Data)
	assert.NotNil(t, resp.Data.Game)
	assert.NotEmpty(t, resp.Data.Achievements)
	assert.NotEmpty(t, resp.Data.Game.LastSyncedAt,
		"RefreshSteamGame should populate last_synced_at")
}
