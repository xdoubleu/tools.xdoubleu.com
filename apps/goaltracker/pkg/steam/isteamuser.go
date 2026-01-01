package steam

import (
	"context"
	"fmt"
)

const BaseImgURL = "http://media.steampowered.com/steamcommunity/public/images/apps/"

type Achievement struct {
	APIName     string `json:"apiname"`
	Achieved    int    `json:"achieved"`
	UnlockTime  int64  `json:"unlocktime"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Game struct {
	AppID                    int    `json:"appid"`
	Name                     string `json:"name"`
	ImgIconURL               string `json:"img_icon_url"`
	ImgLogoURL               string `json:"img_logo_url"`
	HasCommunityVisibleStats bool   `json:"has_community_visible_stats"`
}

func (game Game) GetFullImgIconURL() string {
	return fmt.Sprintf("%s/%d/%s.jpg", BaseImgURL, game.AppID, game.ImgIconURL)
}

func (game Game) GetFullImgLogoURL() string {
	return fmt.Sprintf("%s/%d/%s.jpg", BaseImgURL, game.AppID, game.ImgLogoURL)
}

type OwnedGamesResponse struct {
	Response OwnedGamesResponseData `json:"response"`
}

type OwnedGamesResponseData struct {
	GameCount int    `json:"game_count"`
	Games     []Game `json:"games"`
}

func (client client) GetOwnedGames(
	ctx context.Context,
	steamID string,
) (*OwnedGamesResponse, error) {
	var ownedGamesResponse OwnedGamesResponse

	//nolint:lll //it is what it is
	err := client.sendRequestAPI(
		ctx,
		"IPlayerService/GetOwnedGames/v0001",
		fmt.Sprintf(
			`steamid=%s&include_appinfo=true&include_played_free_games=true&include_free_sub=true`,
			steamID,
		),
		&ownedGamesResponse,
	)
	if err != nil {
		return nil, err
	}

	return &ownedGamesResponse, nil
}

type AchievementsResponse struct {
	PlayerStats PlayerStats `json:"playerstats"`
}

type PlayerStats struct {
	SteamID      string        `json:"steamID"`
	GameName     string        `json:"gameName"`
	Achievements []Achievement `json:"achievements"`
	Success      bool          `json:"success"`
}

func (client client) GetPlayerAchievements(
	ctx context.Context,
	steamID string,
	appID int,
) (*AchievementsResponse, error) {
	var achievementsResponse AchievementsResponse

	err := client.sendRequestAPI(
		ctx,
		"ISteamUserStats/GetPlayerAchievements/v0001",
		fmt.Sprintf("steamid=%s&appid=%d", steamID, appID),
		&achievementsResponse,
	)
	if err != nil {
		return nil, err
	}

	return &achievementsResponse, nil
}

type GetSchemaForGameResponse struct {
	Game GameSchema `json:"game"`
}

type GameSchema struct {
	GameName           string             `json:"gameName"`
	GameVersion        string             `json:"gameVersion"`
	AvailableGameStats AvailableGameStats `json:"availableGameStats"`
}

type AvailableGameStats struct {
	Achievements []AchievementSchema `json:"achievements"`
}

type AchievementSchema struct {
	Name         string `json:"name"`
	DefaultValue int    `json:"defaultValue"`
	DisplayName  string `json:"displayName"`
	Hidden       int    `json:"hidden"`
	Icon         string `json:"icon"`
	IconGray     string `json:"icongray"`
}

func (client client) GetSchemaForGame(
	ctx context.Context,
	appID int,
) (*GetSchemaForGameResponse, error) {
	var getSchemaForGameResponse GetSchemaForGameResponse

	err := client.sendRequestAPI(
		ctx,
		"ISteamUserStats/GetSchemaForGame/v2",
		fmt.Sprintf("appid=%d", appID),
		&getSchemaForGameResponse,
	)
	if err != nil {
		return nil, err
	}

	return &getSchemaForGameResponse, nil
}
