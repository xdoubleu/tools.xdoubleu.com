package services

import (
	"context"

	"tools.xdoubleu.com/apps/games/internal/repositories"
)

// IntegrationsService manages a user's integration settings (Steam user ID).
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
