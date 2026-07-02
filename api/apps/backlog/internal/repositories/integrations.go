package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/backlog/internal/models"
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

// CreateKoboDevice inserts a new device record and returns the persisted model.
func (r *IntegrationsRepository) CreateKoboDevice(
	ctx context.Context,
	userID, name, serial, tokenHash string,
) (models.KoboDevice, error) {
	var d models.KoboDevice
	err := r.db.QueryRow(ctx, `
		INSERT INTO backlog.kobo_devices (user_id, name, serial, token_hash)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		RETURNING id, user_id, name, COALESCE(serial, ''), created_at, last_seen_at
	`, userID, name, serial, tokenHash).Scan(
		&d.ID, &d.UserID, &d.Name, &d.Serial, &d.CreatedAt, &d.LastSeenAt,
	)
	return d, err
}

// ListKoboDevices returns all devices for a user, oldest first.
func (r *IntegrationsRepository) ListKoboDevices(
	ctx context.Context,
	userID string,
) ([]models.KoboDevice, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, COALESCE(serial, ''), created_at, last_seen_at
		FROM backlog.kobo_devices
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.KoboDevice
	for rows.Next() {
		var d models.KoboDevice
		if err = rows.Scan(
			&d.ID, &d.UserID, &d.Name, &d.Serial, &d.CreatedAt, &d.LastSeenAt,
		); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// DeleteKoboDevice removes a device by ID, scoped to the owning user.
func (r *IntegrationsRepository) DeleteKoboDevice(
	ctx context.Context,
	userID string,
	deviceID uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM backlog.kobo_devices WHERE id = $1 AND user_id = $2
	`, deviceID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// GetUserIDByKoboTokenHash looks up the user by token hash and records the
// current time as last_seen_at in one atomic statement.
func (r *IntegrationsRepository) GetUserIDByKoboTokenHash(
	ctx context.Context,
	hash string,
) (string, error) {
	var userID string
	err := r.db.QueryRow(ctx, `
		UPDATE backlog.kobo_devices
		SET last_seen_at = now()
		WHERE token_hash = $1
		RETURNING user_id
	`, hash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", database.ErrResourceNotFound
	}
	return userID, err
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
