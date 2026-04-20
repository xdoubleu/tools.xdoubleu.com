package services

import (
	"context"

	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
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

func (s *IntegrationsService) HasCompletedOnboarding(
	ctx context.Context,
	userID string,
) (bool, error) {
	return s.repo.Exists(ctx, userID)
}

func (s *IntegrationsService) Save(
	ctx context.Context,
	i repositories.UserIntegrations,
) error {
	return s.repo.Upsert(ctx, i)
}
