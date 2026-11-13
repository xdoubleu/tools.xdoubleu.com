package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/kit/database/postgres"
)

// SettingsRepository holds the board-wide switches: one row.
type SettingsRepository struct {
	db postgres.DB
}

// PickupPaused reports whether auto pickup is paused for the whole board.
func (r *SettingsRepository) PickupPaused(ctx context.Context) (bool, error) {
	var paused bool
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE((SELECT pickup_paused FROM factory.board_settings), false)`,
	).Scan(&paused)
	if err != nil {
		return false, postgres.PgxErrorToHTTPError(err)
	}
	return paused, nil
}

// SetPickupPaused pauses or resumes auto pickup for the whole board.
func (r *SettingsRepository) SetPickupPaused(ctx context.Context, paused bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO factory.board_settings (id, pickup_paused) VALUES (true, $1)
		ON CONFLICT (id) DO UPDATE SET pickup_paused = EXCLUDED.pickup_paused`,
		paused)
	return err
}
