package games_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamesv1 "tools.xdoubleu.com/gen/games/v1"
)

func TestSetGameFavourite_UnknownGame(t *testing.T) {
	seedSteamData(t)

	client := newGamesTestClient(t)
	req := connect.NewRequest(&gamesv1.SetGameFavouriteRequest{
		GameId:    999999,
		Favourite: true,
	})
	req.Header().Set("Cookie", accessToken.String())

	_, err := client.SetGameFavourite(context.Background(), req)
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSetGameFavourite_RoundTrip(t *testing.T) {
	seedSteamData(t)
	ctx := context.Background()
	client := newGamesTestClient(t)

	req := connect.NewRequest(&gamesv1.SetGameFavouriteRequest{
		GameId:    1,
		Favourite: true,
	})
	req.Header().Set("Cookie", accessToken.String())
	resp, err := client.SetGameFavourite(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Game)
	assert.True(t, resp.Msg.Game.Favourite)

	// The flag must be visible on reads.
	getReq := connect.NewRequest(&gamesv1.GetSteamGameRequest{GameId: 1})
	getReq.Header().Set("Cookie", accessToken.String())
	getResp, err := client.GetSteamGame(ctx, getReq)
	require.NoError(t, err)
	assert.True(t, getResp.Msg.Data.Game.Favourite)

	// Unset again.
	req = connect.NewRequest(&gamesv1.SetGameFavouriteRequest{
		GameId:    1,
		Favourite: false,
	})
	req.Header().Set("Cookie", accessToken.String())
	resp, err = client.SetGameFavourite(ctx, req)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Game.Favourite)
}

// TestSetGameFavourite_SurvivesSync guards the sync upsert: favourite is
// user-set state and a Steam refresh must never reset it.
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
