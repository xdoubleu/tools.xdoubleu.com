package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// GetProviderIDs returns the user's picked services; none until they save.
func (r *MoviesRepository) GetProviderIDs(
	ctx context.Context,
	userID string,
) ([]int64, error) {
	var ids []int64
	err := r.db.QueryRow(ctx,
		`SELECT provider_ids FROM movies.user_settings WHERE user_id = $1`, userID,
	).Scan(&ids)
	if errors.Is(err, pgx.ErrNoRows) {
		return []int64{}, nil
	}
	return ids, err
}

// SetProviderIDs replaces the user's picked services.
func (r *MoviesRepository) SetProviderIDs(
	ctx context.Context,
	userID string,
	ids []int64,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO movies.user_settings (user_id, provider_ids)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET
			provider_ids = EXCLUDED.provider_ids,
			updated_at = now()`,
		userID, ids,
	)
	return err
}
