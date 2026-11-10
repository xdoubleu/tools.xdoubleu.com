package mocks

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/games/pkg/steam"
)

const (
	mockGameName = "test"
	// MockWishlistAppID is the one app the mock client wishlists.
	MockWishlistAppID = 2
)

type MockSteamClient struct {
}

func NewMockSteamClient() steam.Client {
	return MockSteamClient{}
}

func (client MockSteamClient) GetOwnedGames(
	_ context.Context,
	_ string,
) (*steam.OwnedGamesResponse, error) {
	response := steam.OwnedGamesResponse{
		Response: steam.OwnedGamesResponseData{
			GameCount: 1,
			Games: []steam.Game{
				{
					AppID:                    1,
					Name:                     mockGameName,
					ImgIconURL:               "",
					ImgLogoURL:               "",
					HasCommunityVisibleStats: true,
					PlaytimeForever:          0,
					RtimeLastPlayed:          time.Now().UTC().Unix(),
				},
			},
		},
	}
	return &response, nil
}

func (client MockSteamClient) GetPlayerAchievements(
	_ context.Context,
	steamID string,
	_ int,
) (*steam.AchievementsResponse, error) {
	response := steam.AchievementsResponse{
		PlayerStats: steam.PlayerStats{
			Success:  true,
			SteamID:  steamID,
			GameName: mockGameName,
			Achievements: []steam.Achievement{
				{
					APIName:  "TEST",
					Achieved: 1,
					UnlockTime: int64(
						time.Now().
							UTC().
							Sub(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)).
							Seconds(),
					),
					Name:        mockGameName,
					Description: "Hello, World!",
				},
			},
		},
	}
	return &response, nil
}

func (client MockSteamClient) GetSchemaForGame(
	_ context.Context,
	_ int,
) (*steam.GetSchemaForGameResponse, error) {
	//nolint:exhaustruct //skip
	return &steam.GetSchemaForGameResponse{}, nil
}

func (client MockSteamClient) GetGlobalAchievementPercentagesForApp(
	_ context.Context,
	_ int,
) (*steam.GlobalAchievementPercentagesResponse, error) {
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.GlobalAchievementPercentagesResponse{}
	resp.AchievementPercentages.Achievements = []steam.GlobalAchievementPercent{
		{Name: "TEST", Percent: "42.0"},
	}
	return &resp, nil
}

func (client MockSteamClient) GetWishlist(
	_ context.Context,
	_ string,
) (*steam.WishlistResponse, error) {
	//nolint:exhaustruct //anonymous inner struct initialised via field assignment
	resp := steam.WishlistResponse{}
	resp.Response.Items = []steam.WishlistItem{
		{AppID: MockWishlistAppID, Priority: 1, DateAdded: time.Now().UTC().Unix()},
	}
	return &resp, nil
}

func (client MockSteamClient) GetStoreItems(
	_ context.Context,
	appIDs []int,
) ([]steam.StoreApp, error) {
	apps := make([]steam.StoreApp, len(appIDs))
	for i, id := range appIDs {
		apps[i] = steam.StoreApp{AppID: id, Name: "wished"}
	}
	return apps, nil
}
