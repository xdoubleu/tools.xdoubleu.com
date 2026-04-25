package dtos

type IntegrationsDto struct {
	SteamAPIKey     string `schema:"steam_api_key"`
	SteamUserID     string `schema:"steam_user_id"`
	HardcoverAPIKey string `schema:"hardcover_api_key"`
}
