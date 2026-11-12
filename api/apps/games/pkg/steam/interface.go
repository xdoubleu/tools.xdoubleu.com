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
	GetGlobalAchievementPercentagesForApp(
		ctx context.Context,
		appID int,
	) (*GlobalAchievementPercentagesResponse, error)
}

// WishlistClient is the wishlist half of the Steam API; the real client
// implements it beside Client.
type WishlistClient interface {
	// GetWishlist returns the user's wishlisted apps; a private wishlist
	// comes back empty.
	GetWishlist(ctx context.Context, steamID string) (*WishlistResponse, error)
	// GetStoreItems resolves app ids to their store names and image paths.
	GetStoreItems(ctx context.Context, appIDs []int) ([]StoreApp, error)
}
