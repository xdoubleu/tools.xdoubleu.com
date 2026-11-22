package services

import (
	"context"

	"tools.xdoubleu.com/apps/games/internal/models"
)

// IntegrationsService manages a user's integration settings (Steam user ID).
type IntegrationsService struct {
	repo IntegrationsStore
}

func (s *IntegrationsService) Get(
	ctx context.Context,
	userID string,
) (models.UserIntegrations, error) {
	return s.repo.Get(ctx, userID)
}

func (s *IntegrationsService) Save(
	ctx context.Context,
	i models.UserIntegrations,
) error {
	return s.repo.Upsert(ctx, i)
}
