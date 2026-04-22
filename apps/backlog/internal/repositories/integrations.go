package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
)

type UserIntegrations struct {
	UserID       string
	SteamAPIKey  string
	SteamUserID  string
	GoodreadsURL string
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
		SELECT user_id, steam_api_key, steam_user_id, goodreads_url
		FROM goaltracker.user_integrations
		WHERE user_id = $1
	`, userID).Scan(
		&i.UserID,
		&i.SteamAPIKey,
		&i.SteamUserID,
		&i.GoodreadsURL,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserIntegrations{ //nolint:exhaustruct //other fields default to ""
			UserID: userID,
		}, nil
	}
	return i, err
}

func (r *IntegrationsRepository) Exists(
	ctx context.Context,
	userID string,
) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM goaltracker.user_integrations WHERE user_id = $1)
	`, userID).Scan(&exists)
	return exists, err
}

func (r *IntegrationsRepository) Upsert(
	ctx context.Context,
	i UserIntegrations,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO goaltracker.user_integrations
		    (user_id, steam_api_key, steam_user_id, goodreads_url, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id) DO UPDATE SET
			steam_api_key = EXCLUDED.steam_api_key,
			steam_user_id = EXCLUDED.steam_user_id,
			goodreads_url = EXCLUDED.goodreads_url,
			updated_at    = now()
	`,
		i.UserID,
		i.SteamAPIKey,
		i.SteamUserID,
		i.GoodreadsURL,
	)
	return err
}
