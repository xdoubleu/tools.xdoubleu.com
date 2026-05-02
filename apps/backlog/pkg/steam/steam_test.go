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
	"github.com/xdoubleu/essentia/v4/pkg/logging"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
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

func TestGetFullImgIconURL(t *testing.T) {
	game := steam.Game{ //nolint:exhaustruct //only relevant fields needed
		AppID:      12345,
		ImgIconURL: "abc123",
	}
	assert.Equal(t, steam.BaseImgURL+"/12345/abc123.jpg", game.GetFullImgIconURL())
}

func TestGetFullImgLogoURL(t *testing.T) {
	game := steam.Game{ //nolint:exhaustruct //only relevant fields needed
		AppID:      12345,
		ImgLogoURL: "def456",
	}
	assert.Equal(t, steam.BaseImgURL+"/12345/def456.jpg", game.GetFullImgLogoURL())
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
