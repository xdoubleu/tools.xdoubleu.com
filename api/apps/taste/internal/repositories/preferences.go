package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/database/postgres"
)

type PreferencesRepository struct {
	db postgres.DB
}

func NewPreferencesRepository(db postgres.DB) *PreferencesRepository {
	return &PreferencesRepository{db: db}
}

// EmailNewPicks reports whether userID wants an email for new picks; a user
// who never chose gets the default, on.
func (r *PreferencesRepository) EmailNewPicks(
	ctx context.Context,
	userID string,
) (bool, error) {
	var enabled bool
	err := r.db.QueryRow(ctx,
		`SELECT email_new_picks FROM taste.preferences WHERE user_id = $1`, userID,
	).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return true, postgres.PgxErrorToHTTPError(err)
	}
	return enabled, nil
}

// SetEmailNewPicks stores whether userID wants an email for new picks.
func (r *PreferencesRepository) SetEmailNewPicks(
	ctx context.Context,
	userID string,
	enabled bool,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO taste.preferences (user_id, email_new_picks) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET email_new_picks = EXCLUDED.email_new_picks`,
		userID, enabled)
	return postgres.PgxErrorToHTTPError(err)
}
