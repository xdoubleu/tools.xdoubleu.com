package games_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/pkg/steam"
	"tools.xdoubleu.com/internal/consumption"
)

// wishlistFakeClient is a syncFakeClient with a controllable wishlist.
type wishlistFakeClient struct {
	syncFakeClient
	wished   []steam.WishlistItem
	err      error
	storeErr error
}

func (c *wishlistFakeClient) GetWishlist(
	context.Context,
	string,
) (*steam.WishlistResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	//nolint:exhaustruct //anonymous inner struct set by field assignment
	resp := &steam.WishlistResponse{}
	resp.Response.Items = c.wished
	return resp, nil
}

func (c *wishlistFakeClient) GetStoreItems(
	_ context.Context,
	ids []int,
) ([]steam.StoreApp, error) {
	if c.storeErr != nil {
		return nil, c.storeErr
	}
	apps := []steam.StoreApp{}
	for _, id := range ids {
		if id == 9003 { // stays unresolved
			continue
		}
		app := steam.StoreApp{
			AppID:  id,
			Name:   "Wished",
			Assets: steam.StoreAssets{MainCapsule: ""},
		}
		if id == 9000 || id == 9001 { // newer apps: hashed assets only
			app.Assets.MainCapsule = "hash/capsule_616x353.jpg"
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func TestSyncUser_Wishlist(t *testing.T) {
	ctx := context.Background()
	const user = "sync-wishlist-user"
	added := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	client := &wishlistFakeClient{
		syncFakeClient: syncFakeClient{
			//nolint:exhaustruct //only required fields
			games:     []steam.Game{{AppID: 9000, Name: "Owned"}},
			playerAch: map[int][]steam.Achievement{},
			schemaErr: map[int]bool{},
		},
		wished: []steam.WishlistItem{
			{AppID: 9001, Priority: 2, DateAdded: added.Unix()},
			{AppID: 9002, Priority: 0, DateAdded: 0},
			{AppID: 9003, Priority: 1, DateAdded: 0},
			{AppID: 9000, Priority: 3, DateAdded: 0}, // owned: skipped
		},
		err:      nil,
		storeErr: nil,
	}
	app := newSyncTestApp(t, user, client)
	t.Cleanup(func() {
		_, _ = testDB.Exec(ctx,
			`DELETE FROM games.steam_wishlist WHERE user_id = $1`, user)
	})
	svc := app.TestSeeds().Services.Steam

	require.NoError(t, svc.SyncUser(ctx, user))
	items, err := svc.GetWishlist(ctx, user)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, 9003, items[0].ID, "priority 1 first")
	assert.Equal(t, "Steam app 9003", items[0].Name, "unresolved name falls back")
	assert.Equal(t, 9001, items[1].ID)
	require.NotNil(t, items[1].AddedAt)
	assert.True(t, added.Equal(*items[1].AddedAt))
	assert.Equal(t, 9002, items[2].ID, "unranked last")
	hashed := func(id int) string {
		return fmt.Sprintf(
			"https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/"+
				"%d/hash/capsule_616x353.jpg", id)
	}
	assert.Equal(t, hashed(9001), items[1].ImageURL, "hashed capsule")
	assert.Equal(t, steam.CapsuleURL(9002), items[2].ImageURL, "unhashed fallback")

	owned, err := svc.GetGameByID(ctx, 9000, user)
	require.NoError(t, err)
	assert.Equal(t, hashed(9000), owned.ImageURL)

	events, err := app.ListConsumption(ctx, user)
	require.NoError(t, err)
	wished := 0
	for _, e := range events {
		if e.Reason == "on your wishlist" {
			wished++
			assert.Equal(t, consumption.Positive, e.Signal)
		}
	}
	assert.Equal(t, 3, wished)

	// A failed fetch keeps the stored list.
	client.err = errors.New("steam down")
	require.NoError(t, svc.SyncUser(ctx, user))
	items, err = svc.GetWishlist(ctx, user)
	require.NoError(t, err)
	assert.Len(t, items, 3)

	// A failed artwork lookup keeps the stored hashed image of an owned game.
	client.storeErr = errors.New("store down")
	require.NoError(t, svc.SyncUser(ctx, user))
	owned, err = svc.GetGameByID(ctx, 9000, user)
	require.NoError(t, err)
	assert.Equal(t, hashed(9000), owned.ImageURL)
	client.storeErr = nil

	// An entry that left the wishlist is removed.
	client.err = nil
	client.wished = client.wished[:1]
	require.NoError(t, svc.SyncUser(ctx, user))
	items, err = svc.GetWishlist(ctx, user)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, 9001, items[0].ID)
}
