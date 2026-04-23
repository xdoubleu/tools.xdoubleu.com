package dtos

type IntegrationsDto struct {
	SteamAPIKey  string `schema:"steam_api_key"`
	SteamUserID  string `schema:"steam_user_id"`
	GoodreadsURL string `schema:"goodreads_url"`
}
