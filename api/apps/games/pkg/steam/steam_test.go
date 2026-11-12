package steam_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/games/pkg/steam"
	"tools.xdoubleu.com/internal/logging"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(
	r *http.Request,
) (*http.Response, error) {
	return f(r)
}

func mockSteamServer(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(handler)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			req2 := req.Clone(req.Context())
			parsed, _ := url.Parse(srv.URL)
			req2.URL.Scheme = parsed.Scheme
			req2.URL.Host = parsed.Host
			return orig.RoundTrip(req2)
		},
	)
	return func() {
		http.DefaultTransport = orig
		srv.Close()
	}
}

func TestNew(t *testing.T) {
	client := steam.New(logging.NewNopLogger(), "test-api-key")
	assert.NotNil(t, client)
}

func TestCapsuleURL(t *testing.T) {
	assert.Equal(
		t,
		"https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/"+
			"12345/capsule_616x353.jpg",
		steam.CapsuleURL(12345),
	)
}

func TestGetOwnedGames(t *testing.T) {
	want := steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{ //nolint:exhaustruct //only relevant fields needed
					AppID: 440,
					Name:  "Team Fortress 2",
				},
			},
		},
	}

	cleanup := mockSteamServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	})
	defer cleanup()

	client := steam.New(logging.NewNopLogger(), "test-key")
	resp, err := client.GetOwnedGames(context.Background(), "76561197960435530")
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Response.GameCount)
	assert.Equal(t, "Team Fortress 2", resp.Response.Games[0].Name)
}

func TestGetOwnedGamesServerError(t *testing.T) {
	cleanup := mockSteamServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	defer cleanup()

	client := steam.New(logging.NewNopLogger(), "bad-key")
	_, err := client.GetOwnedGames(context.Background(), "76561197960435530")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestGetPlayerAchievements(t *testing.T) {
	want := steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			SteamID:  "76561197960435530",
			GameName: "TF2",
			Success:  true,
			Achievements: []steam.Achievement{
				{ //nolint:exhaustruct //only relevant fields needed
					APIName:  "ACH_WIN_ONE_GAME",
					Achieved: 1,
				},
			},
		},
	}

	cleanup := mockSteamServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	})
	defer cleanup()

	client := steam.New(logging.NewNopLogger(), "test-key")
	resp, err := client.GetPlayerAchievements(
		context.Background(),
		"76561197960435530",
		440,
	)
	require.NoError(t, err)
	assert.True(t, resp.PlayerStats.Success)
	assert.Len(t, resp.PlayerStats.Achievements, 1)
}

func TestGetSchemaForGame(t *testing.T) {
	want := steam.GetSchemaForGameResponse{
		Game: steam.GameSchema{ //nolint:exhaustruct //only relevant fields needed
			GameName: "Team Fortress 2",
			AvailableGameStats: steam.AvailableGameStats{
				Achievements: []steam.AchievementSchema{
					{ //nolint:exhaustruct //only relevant fields needed
						Name:        "ACH_WIN",
						DisplayName: "First Win",
					},
				},
			},
		},
	}

	cleanup := mockSteamServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	})
	defer cleanup()

	client := steam.New(logging.NewNopLogger(), "test-key")
	resp, err := client.GetSchemaForGame(context.Background(), 440)
	require.NoError(t, err)
	assert.Equal(t, "Team Fortress 2", resp.Game.GameName)
	assert.Len(t, resp.Game.AvailableGameStats.Achievements, 1)
}

func TestGetSchemaForGameInvalidJSON(t *testing.T) {
	cleanup := mockSteamServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-valid-json`))
	})
	defer cleanup()

	client := steam.New(logging.NewNopLogger(), "test-key")
	_, err := client.GetSchemaForGame(context.Background(), 440)
	require.Error(t, err)
}

func TestGetWishlist(t *testing.T) {
	defer mockSteamServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/IWishlistService/GetWishlist/v1", r.URL.Path)
		assert.Equal(t, "42", r.URL.Query().Get("steamid"))
		_, _ = w.Write([]byte(
			`{"response":{"items":[{"appid":10,"priority":1,"date_added":5}]}}`))
	})()

	client := newWishlistClient(t)
	resp, err := client.GetWishlist(context.Background(), "42")
	require.NoError(t, err)
	require.Len(t, resp.Response.Items, 1)
	assert.Equal(t, 10, resp.Response.Items[0].AppID)
	assert.EqualValues(t, 5, resp.Response.Items[0].DateAdded)
}

func TestGetWishlist_Error(t *testing.T) {
	defer mockSteamServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})()

	client := newWishlistClient(t)
	_, err := client.GetWishlist(context.Background(), "42")
	require.Error(t, err)
}

func TestGetStoreItems(t *testing.T) {
	defer mockSteamServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/IStoreBrowseService/GetItems/v1", r.URL.Path)
		assert.Contains(t, r.URL.Query().Get("input_json"), `"appid":10`)
		assert.Contains(t, r.URL.Query().Get("input_json"), `"include_assets":true`)
		_, _ = w.Write([]byte(
			`{"response":{"store_items":[{"appid":10,"name":"Ten",` +
				`"assets":{"main_capsule":"abc/capsule_616x353.jpg"}}]}}`))
	})()

	client := newWishlistClient(t)
	apps, err := client.GetStoreItems(context.Background(), []int{10})
	require.NoError(t, err)
	assert.Equal(t, []steam.StoreApp{{
		AppID:  10,
		Name:   "Ten",
		Assets: steam.StoreAssets{MainCapsule: "abc/capsule_616x353.jpg"},
	}}, apps)
	assert.Equal(t,
		"https://shared.cloudflare.steamstatic.com/store_item_assets/steam/apps/"+
			"10/abc/capsule_616x353.jpg",
		apps[0].ImageURL())
	unhashed := steam.StoreApp{
		AppID:  11,
		Name:   "",
		Assets: steam.StoreAssets{MainCapsule: ""},
	}
	assert.Equal(t, steam.CapsuleURL(11), unhashed.ImageURL())

	apps, err = client.GetStoreItems(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, apps)
}

func TestGetStoreItems_Error(t *testing.T) {
	defer mockSteamServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})()

	client := newWishlistClient(t)
	_, err := client.GetStoreItems(context.Background(), []int{10})
	require.Error(t, err)
}

func newWishlistClient(t *testing.T) steam.WishlistClient {
	t.Helper()
	client, ok := steam.New(logging.NewNopLogger(), "k").(steam.WishlistClient)
	require.True(t, ok)
	return client
}
