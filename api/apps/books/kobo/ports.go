package kobo

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// deviceStore is the Kobo device storage the service manages.
type deviceStore interface {
	CreateKoboDevice(
		ctx context.Context,
		userID,
		name,
		serial,
		tokenHash string,
	) (models.KoboDevice, error)
	DeleteKoboDevice(ctx context.Context, userID string, deviceID uuid.UUID) error
	GetKoboAuthByTokenHash(ctx context.Context, hash string) (string, string, error)
	GetKoboDevice(
		ctx context.Context,
		userID string,
		deviceID uuid.UUID,
	) (models.KoboDevice, error)
	GetLastSeenAt(ctx context.Context, userID string) (*time.Time, error)
	ListKoboDevices(ctx context.Context, userID string) ([]models.KoboDevice, error)
}
