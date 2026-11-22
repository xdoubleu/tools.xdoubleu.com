package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/games/internal/models"
)

// SaveLibrary persists a full library sync in one transaction: the games,
// each fetched game's achievements and the progress graph.
func (repo *SteamRepository) SaveLibrary(
	ctx context.Context,
	userID string,
	games map[int]*models.Game,
	achievements map[int][]models.Achievement,
	labels, values []string,
) error {
	progress := &ProgressRepository{db: repo.db}
	return repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := repo.UpsertGames(ctx, tx, games, userID); err != nil {
			return err
		}
		for gameID, rows := range achievements {
			if err := repo.ReplaceAchievements(
				ctx, tx, userID, gameID, rows,
			); err != nil {
				return err
			}
		}
		return progress.UpsertTx(ctx, tx, userID, labels, values)
	})
}

// SaveGame persists one game's refresh in one transaction: its achievements,
// the game row and the progress graph.
func (repo *SteamRepository) SaveGame(
	ctx context.Context,
	userID string,
	game *models.Game,
	achievements []models.Achievement,
	labels, values []string,
) error {
	progress := &ProgressRepository{db: repo.db}
	return repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := repo.ReplaceAchievements(
			ctx, tx, userID, game.ID, achievements,
		); err != nil {
			return err
		}
		if err := repo.UpsertGames(
			ctx, tx, map[int]*models.Game{game.ID: game}, userID,
		); err != nil {
			return err
		}
		return progress.UpsertTx(ctx, tx, userID, labels, values)
	})
}
