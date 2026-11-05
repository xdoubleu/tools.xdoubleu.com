package repositories

import (
	"context"

	"tools.xdoubleu.com/internal/database/postgres"
)

// SettingsRepository holds the board-wide switches: one row.
type SettingsRepository struct {
	db postgres.DB
}

// PickupPaused reports whether auto pickup is paused for the whole board.
func (r *SettingsRepository) PickupPaused(ctx context.Context) (bool, error) {
	var paused bool
	err := r.db.QueryRow(ctx,
		`SELECT pickup_paused FROM factory.board_settings`).Scan(&paused)
	if err != nil {
		return false, postgres.PgxErrorToHTTPError(err)
	}
	return paused, nil
}

// SetPickupPaused pauses or resumes auto pickup for the whole board.
func (r *SettingsRepository) SetPickupPaused(ctx context.Context, paused bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE factory.board_settings SET pickup_paused = $1`, paused)
	return err
}
