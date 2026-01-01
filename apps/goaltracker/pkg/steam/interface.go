package steam

import "context"

type Client interface {
	GetOwnedGames(ctx context.Context, steamID string) (*OwnedGamesResponse, error)
	GetPlayerAchievements(
		ctx context.Context,
		steamID string,
		appID int,
	) (*AchievementsResponse, error)
	GetSchemaForGame(ctx context.Context, appID int) (*GetSchemaForGameResponse, error)
}
