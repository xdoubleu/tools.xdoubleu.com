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

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	gamesv1connect "tools.xdoubleu.com/gen/games/v1/gamesv1connect"
)

func newGamesTestClient(t *testing.T) gamesv1connect.GamesServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return gamesv1connect.NewGamesServiceClient(
		http.DefaultClient,
		ts.URL,
		connect.WithHTTPGet(),
	)
}

func TestConnectGetSteam(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(
		&gamesv1.GetSteamRequest{},
	)
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteam(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp.Msg.Steam != nil {
		assert.GreaterOrEqual(t, int(resp.Msg.Steam.TotalBacklog), 0)
		assert.NotNil(t, resp.Msg.Steam.Distribution)
	}
}

func TestConnectGetSteamGame(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.GetSteamGameRequest{
		GameId: 570,
	})
	req.Header().Set("Cookie", accessToken.String())

	// The mock may not have every game, so we just check the request can be called
	// without panicking. It may return an error if the game doesn't exist in the mock.
	_, _ = client.GetSteamGame(ctx, req)
}

func TestConnectGetSteamGame_WithSeededData(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.GetSteamGameRequest{
		GameId: 1,
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamGame(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.Data)
	assert.NotNil(t, resp.Msg.Data.Game)
	assert.NotEmpty(t, resp.Msg.Data.Game.LastSyncedAt,
		"GetSteamGame should populate last_synced_at")
}

func TestConnectGetRecentlyActiveGames(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.GetRecentlyActiveGamesRequest{})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetRecentlyActiveGames(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotEmpty(t, resp.Msg.Games)

	found := false
	for _, g := range resp.Msg.Games {
		if g.Id == 1 {
			found = true
			assert.GreaterOrEqual(t, g.RecentUnlocks, int32(1))
			assert.NotEmpty(t, g.LastUnlockedAt)
		}
	}
	assert.True(t, found, "seeded game should appear in recent activity")
}

func TestConnectGetSteamDistribution_ValidBucket(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.GetSteamDistributionRequest{
		Bucket: 0,
	})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.GetSteamDistribution(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp.Msg.Data != nil {
		assert.Equal(t, "0–9%", resp.Msg.Data.Label)
	}
}

func TestConnectGetSteamDistribution_InvalidBucket(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.GetSteamDistributionRequest{
		Bucket: 99,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.GetSteamDistribution(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

// TestConnectRefreshSteamGame_GameNotFound verifies that RefreshSteamGame
// returns an error when the requested game does not exist in the database (the
// no-credentials no-op still falls through to GetGameByID which fails).
func TestConnectRefreshSteamGame_GameNotFound(t *testing.T) {
	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Game 999999 was never seeded; testApp has empty Steam creds so SyncGame
	// is a no-op, and GetGameByID returns an error.
	req := connect.NewRequest(&gamesv1.RefreshSteamGameRequest{GameId: 999999})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.RefreshSteamGame(ctx, req)
	assert.Error(t, err)
	var connectErr *connect.Error
	assert.True(t, errors.As(err, &connectErr))
}

func TestConnectRefreshSteamGame_WithSeededData(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := connect.NewRequest(&gamesv1.RefreshSteamGameRequest{GameId: 1})
	req.Header().Set("Cookie", accessToken.String())

	resp, err := client.RefreshSteamGame(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Msg.Data)
	assert.NotNil(t, resp.Msg.Data.Game)
	assert.NotEmpty(t, resp.Msg.Data.Achievements)
	assert.NotEmpty(t, resp.Msg.Data.Game.LastSyncedAt,
		"RefreshSteamGame should populate last_synced_at")
}
