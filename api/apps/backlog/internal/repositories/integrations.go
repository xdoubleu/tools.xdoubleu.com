package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
)

type UserIntegrations struct {
	UserID      string
	SteamUserID string
}

type IntegrationsRepository struct {
	db postgres.DB
}

func (r *IntegrationsRepository) Get(
	ctx context.Context,
	userID string,
) (UserIntegrations, error) {
	var i UserIntegrations
	err := r.db.QueryRow(ctx, `
		SELECT user_id, steam_user_id
		FROM backlog.user_integrations
		WHERE user_id = $1
	`, userID).Scan(
		&i.UserID,
		&i.SteamUserID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserIntegrations{ //nolint:exhaustruct //other fields default to ""
			UserID: userID,
		}, nil
	}
	return i, err
}

func (r *IntegrationsRepository) Upsert(
	ctx context.Context,
	i UserIntegrations,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO backlog.user_integrations
		    (user_id, steam_user_id, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (user_id) DO UPDATE SET
			steam_user_id = EXCLUDED.steam_user_id,
			updated_at    = now()
	`,
		i.UserID,
		i.SteamUserID,
	)
	return err
}
