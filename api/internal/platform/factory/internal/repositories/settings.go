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
		SELECT COALESCE((SELECT pickup_paused FROM factory.board_settings -- single-row
		), false)`,
	).Scan(&paused)
	if err != nil {
		return false, postgres.PgxErrorToHTTPError(err)
	}
	return paused, nil
}

// ImageDigest is the job image digest last seen, empty if none yet.
func (r *SettingsRepository) ImageDigest(ctx context.Context) (string, error) {
	var digest string
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE((SELECT image_digest FROM factory.board_settings -- single-row
		), '')`,
	).Scan(&digest)
	if err != nil {
		return "", postgres.PgxErrorToHTTPError(err)
	}
	return digest, nil
}

// SetImageDigest records the job image digest last seen.
func (r *SettingsRepository) SetImageDigest(ctx context.Context, digest string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO factory.board_settings (id, image_digest) VALUES (true, $1)
		ON CONFLICT (id) DO UPDATE SET image_digest = EXCLUDED.image_digest`,
		digest)
	return err
}

// SetPickupPaused pauses or resumes auto pickup for the whole board.
func (r *SettingsRepository) SetPickupPaused(ctx context.Context, paused bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO factory.board_settings (id, pickup_paused) VALUES (true, $1)
		ON CONFLICT (id) DO UPDATE SET pickup_paused = EXCLUDED.pickup_paused`,
		paused)
	return err
}

// PathRulesBackfilled reports whether the one-off move of self-checked Testing
// tasks to Done has run.
func (r *SettingsRepository) PathRulesBackfilled(ctx context.Context) (bool, error) {
	var done bool
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE((SELECT path_rules_backfilled
			FROM factory.board_settings -- single-row
		), false)`,
	).Scan(&done)
	if err != nil {
		return false, postgres.PgxErrorToHTTPError(err)
	}
	return done, nil
}

// SetPathRulesBackfilled records that the one-off backfill has run.
func (r *SettingsRepository) SetPathRulesBackfilled(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO factory.board_settings (id, path_rules_backfilled) VALUES (true, true)
		ON CONFLICT (id) DO UPDATE SET path_rules_backfilled = true`)
	return err
}
