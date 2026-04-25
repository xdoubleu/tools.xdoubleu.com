package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"tools.xdoubleu.com/internal/crypto"
)

type UserIntegrations struct {
	UserID          string
	SteamAPIKey     string
	SteamUserID     string
	HardcoverAPIKey string
}

type IntegrationsRepository struct {
	db            postgres.DB
	encryptionKey []byte
}

func (r *IntegrationsRepository) Get(
	ctx context.Context,
	userID string,
) (UserIntegrations, error) {
	var i UserIntegrations
	err := r.db.QueryRow(ctx, `
		SELECT user_id, steam_api_key, steam_user_id, hardcover_api_key
		FROM backlog.user_integrations
		WHERE user_id = $1
	`, userID).Scan(
		&i.UserID,
		&i.SteamAPIKey,
		&i.SteamUserID,
		&i.HardcoverAPIKey,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserIntegrations{ //nolint:exhaustruct //other fields default to ""
			UserID: userID,
		}, nil
	}
	if err != nil {
		return i, err
	}

	i.SteamAPIKey, err = crypto.Decrypt(r.encryptionKey, i.SteamAPIKey)
	if err != nil {
		return i, err
	}

	i.HardcoverAPIKey, err = crypto.Decrypt(r.encryptionKey, i.HardcoverAPIKey)
	if err != nil {
		return i, err
	}

	return i, nil
}

func (r *IntegrationsRepository) Exists(
	ctx context.Context,
	userID string,
) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM backlog.user_integrations WHERE user_id = $1)
	`, userID).Scan(&exists)
	return exists, err
}

func (r *IntegrationsRepository) Upsert(
	ctx context.Context,
	i UserIntegrations,
) error {
	encSteamKey, err := crypto.Encrypt(r.encryptionKey, i.SteamAPIKey)
	if err != nil {
		return err
	}

	encHardcoverKey, err := crypto.Encrypt(r.encryptionKey, i.HardcoverAPIKey)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO backlog.user_integrations
		    (user_id, steam_api_key, steam_user_id, hardcover_api_key, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (user_id) DO UPDATE SET
			steam_api_key     = EXCLUDED.steam_api_key,
			steam_user_id     = EXCLUDED.steam_user_id,
			hardcover_api_key = EXCLUDED.hardcover_api_key,
			updated_at        = now()
	`,
		i.UserID,
		encSteamKey,
		i.SteamUserID,
		encHardcoverKey,
	)
	return err
}
