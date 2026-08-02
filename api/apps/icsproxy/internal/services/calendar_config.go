package services

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
)

type CalendarService struct {
	logger *slog.Logger
	repo   *repositories.CalendarRepository
}

func (s *CalendarService) SaveConfig(
	ctx context.Context,
	cfg models.FilterConfig,
) error {
	return s.repo.UpsertFilterConfig(ctx, cfg)
}

func (s *CalendarService) LoadConfig(
	ctx context.Context,
	token string,
) (models.FilterConfig, bool) {
	return s.repo.GetFilterConfig(ctx, token)
}

func (s *CalendarService) ListConfigs(
	ctx context.Context,
	userID string,
	limit int32,
	offset int32,
) ([]models.FilterConfig, bool, error) {
	return s.repo.ListFilterConfigs(ctx, userID, limit, offset)
}

func (s *CalendarService) ListConfigSummaries(
	ctx context.Context,
	userID string,
) ([]repositories.FilterSummary, error) {
	return s.repo.ListFilterSummaries(ctx, userID)
}

func (s *CalendarService) DeleteConfig(
	ctx context.Context,
	token string,
	userID string,
) error {
	return s.repo.DeleteFilterConfig(ctx, token, userID)
}
