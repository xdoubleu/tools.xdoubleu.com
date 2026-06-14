package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/backlog/internal/models"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
)

type IntegrationsService struct {
	repo *repositories.IntegrationsRepository
}

func (s *IntegrationsService) Get(
	ctx context.Context,
	userID string,
) (repositories.UserIntegrations, error) {
	return s.repo.Get(ctx, userID)
}

func (s *IntegrationsService) Save(
	ctx context.Context,
	i repositories.UserIntegrations,
) error {
	return s.repo.Upsert(ctx, i)
}

// koboTokenBytes is the number of random bytes for a Kobo sync token
// (256 bits of entropy — URL-safe base64 yields a 43-character string).
const koboTokenBytes = 32

// RegisterKoboDevice generates a new high-entropy random token, stores only
// its sha256 hash (never the raw token), and returns the persisted device
// record together with the raw token for one-time display.
func (s *IntegrationsService) RegisterKoboDevice(
	ctx context.Context,
	userID, name, serial string,
) (models.KoboDevice, string, error) {
	raw := make([]byte, koboTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return models.KoboDevice{}, "", err
	}
	rawToken := base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(h[:])
	device, err := s.repo.CreateKoboDevice(ctx, userID, name, serial, hash)
	if err != nil {
		return models.KoboDevice{}, "", err
	}
	return device, rawToken, nil
}

// ListKoboDevices returns all registered devices for a user.
func (s *IntegrationsService) ListKoboDevices(
	ctx context.Context,
	userID string,
) ([]models.KoboDevice, error) {
	return s.repo.ListKoboDevices(ctx, userID)
}

// DisconnectKoboDevice deletes the device record, revoking its token.
func (s *IntegrationsService) DisconnectKoboDevice(
	ctx context.Context,
	userID string,
	deviceID uuid.UUID,
) error {
	return s.repo.DeleteKoboDevice(ctx, userID, deviceID)
}

// GetUserIDByKoboTokenHash looks up the user by a pre-hashed token for
// authenticating Kobo sync requests.
func (s *IntegrationsService) GetUserIDByKoboTokenHash(
	ctx context.Context,
	hash string,
) (string, error) {
	return s.repo.GetUserIDByKoboTokenHash(ctx, hash)
}
