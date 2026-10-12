package games_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
	"tools.xdoubleu.com/internal/testhelper"
)

func TestSetGameFavourite_UnknownGame(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	req := &gamesv1.SetGameFavouriteRequest{
		GameId:    999999,
		Favourite: true,
	}

	_, err := client.SetGameFavourite(
		testhelper.WithCookie(context.Background(), accessToken.String()),
		req,
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSetGameFavourite_RoundTrip(t *testing.T) {
	seedSteamData(t)
	ctx := context.Background()
	client := newGamesTestClient(t)

	req := &gamesv1.SetGameFavouriteRequest{
		GameId:    1,
		Favourite: true,
	}
	resp, err := client.SetGameFavourite(
		testhelper.WithCookie(ctx, accessToken.String()),
		req,
	)
	require.NoError(t, err)
	require.NotNil(t, resp.Game)
	assert.True(t, resp.Game.Favourite)

	getReq := &gamesv1.GetSteamGameRequest{GameId: 1}
	getResp, err := client.GetSteamGame(
		testhelper.WithCookie(ctx, accessToken.String()),
		getReq,
	)
	require.NoError(t, err)
	assert.True(t, getResp.Data.Game.Favourite)

	req = &gamesv1.SetGameFavouriteRequest{
		GameId:    1,
		Favourite: false,
	}
	resp, err = client.SetGameFavourite(
		testhelper.WithCookie(ctx, accessToken.String()),
		req,
	)
	require.NoError(t, err)
	assert.False(t, resp.Game.Favourite)
}

// TestSetGameFavourite_SurvivesSync: a Steam refresh never resets favourite.
func TestSetGameFavourite_SurvivesSync(t *testing.T) {
	seedSteamData(t)
	ctx := context.Background()

	require.NoError(t,
		testApp.Services.Steam.SetFavourite(ctx, userID, 1, true))

	// Re-run the full library sync, which upserts every game row.
	require.NoError(t, testApp.Services.Steam.SyncUser(ctx, userID))

	game, err := testApp.Services.Steam.GetGameByID(ctx, 1, userID)
	require.NoError(t, err)
	assert.True(t, game.Favourite, "favourite must survive a Steam sync")
}
