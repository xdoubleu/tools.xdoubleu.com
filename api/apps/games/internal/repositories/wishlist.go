package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/games/internal/models"
	"tools.xdoubleu.com/internal/kit/database/postgres"
)

// GetWishlist returns userID's wishlist, highest Steam priority first
// (priority 0 means unranked and sorts last).
func (repo *SteamRepository) GetWishlist(
	ctx context.Context,
	userID string,
) ([]models.WishlistItem, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT game_id, name, image_url, priority, added_at
		FROM games.steam_wishlist
		WHERE user_id = $1
		ORDER BY (priority = 0), priority, name
	`, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	items := []models.WishlistItem{}
	for rows.Next() {
		var it models.WishlistItem
		if err = rows.Scan(
			&it.ID, &it.Name, &it.ImageURL, &it.Priority, &it.AddedAt,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		items = append(items, it)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return items, nil
}

// ReplaceWishlist makes items userID's whole wishlist, in one transaction.
func (repo *SteamRepository) ReplaceWishlist(
	ctx context.Context,
	userID string,
	items []models.WishlistItem,
) error {
	return repo.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(
			ctx, `DELETE FROM games.steam_wishlist WHERE user_id = $1`, userID,
		); err != nil {
			return postgres.PgxErrorToHTTPError(err)
		}

		//nolint:exhaustruct //fields are optional
		b := &pgx.Batch{}
		for _, it := range items {
			b.Queue(`
				INSERT INTO games.steam_wishlist
				    (game_id, user_id, name, image_url, priority, added_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, it.ID, userID, it.Name, it.ImageURL, it.Priority, it.AddedAt)
		}
		if err := tx.SendBatch(ctx, b).Close(); err != nil {
			return postgres.PgxErrorToHTTPError(err)
		}
		return nil
	})
}
