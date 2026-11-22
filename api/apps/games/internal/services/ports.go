package services

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/apps/internal/progresshistory"
)

// SteamStore is the storage the Steam service needs.
type SteamStore interface {
	GetAllGames(ctx context.Context, userID string) ([]models.Game, error)
	GetAveragedGames(ctx context.Context, userID string) ([]models.Game, error)
	GetDelisted(ctx context.Context, userID string) ([]models.Game, error)
	GetBacklog(ctx context.Context, userID string) ([]models.Game, error)
	GetInProgress(ctx context.Context, userID string) ([]models.Game, error)
	GetCompleted(ctx context.Context, userID string) ([]models.Game, error)
	GetRecentlyActiveGames(
		ctx context.Context,
		userID string,
		limit int,
	) ([]models.RecentGame, error)
	GetGameByID(ctx context.Context, gameID int, userID string) (*models.Game, error)
	GetAchievementsForGames(
		ctx context.Context,
		gameIDs []int,
		userID string,
	) (map[int][]models.Achievement, error)
	GetLastSyncedAt(ctx context.Context, userID string) (*time.Time, error)
	SetFavourite(ctx context.Context, userID string, gameID int, favourite bool) error
	GetWishlist(ctx context.Context, userID string) ([]models.WishlistItem, error)
	ReplaceWishlist(ctx context.Context, userID string, items []models.WishlistItem) error
	// SaveLibrary atomically stores a library sync: games, the fetched
	// games' achievements and the progress graph.
	SaveLibrary(
		ctx context.Context,
		userID string,
		games map[int]*models.Game,
		achievements map[int][]models.Achievement,
		labels, values []string,
	) error
	// SaveGame atomically stores one game's refresh and the progress graph.
	SaveGame(
		ctx context.Context,
		userID string,
		game *models.Game,
		achievements []models.Achievement,
		labels, values []string,
	) error
}

// ProgressStore is the progress-history storage plus the latest value.
type ProgressStore interface {
	progresshistory.Repository
	GetLatest(ctx context.Context, userID string) (string, error)
}

// IntegrationsStore stores a user's integration settings.
type IntegrationsStore interface {
	Get(ctx context.Context, userID string) (models.UserIntegrations, error)
	Upsert(ctx context.Context, i models.UserIntegrations) error
}

// Stores are the storage ports the services are built on.
type Stores struct {
	Steam        SteamStore
	Progress     ProgressStore
	Integrations IntegrationsStore
}
