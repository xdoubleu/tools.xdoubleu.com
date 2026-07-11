package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/books/internal/models"
)

type KoboDevicesRepository struct {
	db postgres.DB
}

// CreateKoboDevice inserts a new device record and returns the persisted model.
func (r *KoboDevicesRepository) CreateKoboDevice(
	ctx context.Context,
	userID, name, serial, tokenHash string,
) (models.KoboDevice, error) {
	var d models.KoboDevice
	err := r.db.QueryRow(ctx, `
		INSERT INTO books.kobo_devices (user_id, name, serial, token_hash)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		RETURNING id, user_id, name, COALESCE(serial, ''), created_at, last_seen_at
	`, userID, name, serial, tokenHash).Scan(
		&d.ID, &d.UserID, &d.Name, &d.Serial, &d.CreatedAt, &d.LastSeenAt,
	)
	return d, err
}

// ListKoboDevices returns all devices for a user, oldest first.
func (r *KoboDevicesRepository) ListKoboDevices(
	ctx context.Context,
	userID string,
) ([]models.KoboDevice, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, name, COALESCE(serial, ''), created_at, last_seen_at
		FROM books.kobo_devices
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

// GetKoboDevice returns a single device by ID, scoped to the owning user.
// Returns database.ErrResourceNotFound when no matching row exists.
func (r *KoboDevicesRepository) GetKoboDevice(
	ctx context.Context,
	userID string,
	deviceID uuid.UUID,
) (models.KoboDevice, error) {
	var d models.KoboDevice
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, name, COALESCE(serial, ''), created_at, last_seen_at
		FROM books.kobo_devices
		WHERE id = $1 AND user_id = $2
	`, deviceID, userID).Scan(
		&d.ID, &d.UserID, &d.Name, &d.Serial, &d.CreatedAt, &d.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.KoboDevice{}, database.ErrResourceNotFound
	}
	return d, err
}

// DeleteKoboDevice removes a device by ID, scoped to the owning user.
func (r *KoboDevicesRepository) DeleteKoboDevice(
	ctx context.Context,
	userID string,
	deviceID uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM books.kobo_devices WHERE id = $1 AND user_id = $2
	`, deviceID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// GetKoboAuthByTokenHash looks up the owning user and device ID by token hash
// and records the current time as last_seen_at in one atomic statement.
func (r *KoboDevicesRepository) GetKoboAuthByTokenHash(
	ctx context.Context,
	hash string,
) (string, string, error) {
	var userID, deviceID string
	err := r.db.QueryRow(ctx, `
		UPDATE books.kobo_devices
		SET last_seen_at = now()
		WHERE token_hash = $1
		RETURNING user_id, id
	`, hash).Scan(&userID, &deviceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", database.ErrResourceNotFound
	}
	return userID, deviceID, err
}
