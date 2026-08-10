package services

import (
	"context"
	"errors"
	"log/slog"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
)

var (
	ErrConfigNotFound     = errors.New("config not found")
	ErrConfigAccessDenied = errors.New("permission denied")
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

// GetConfigWithEvents loads the config for token, verifies userID owns it,
// and returns it together with its source calendar's current events.
func (s *CalendarService) GetConfigWithEvents(
	ctx context.Context,
	token string,
	userID string,
) (models.FilterConfig, []models.EventInfo, error) {
	cfg, ok := s.LoadConfig(ctx, token)
	if err := authorizeConfigAccess(cfg, ok, userID); err != nil {
		return models.FilterConfig{}, nil, err
	}

	events, err := s.PreviewEvents(ctx, cfg.SourceURL)
	if err != nil {
		return models.FilterConfig{}, nil, err
	}

	return cfg, events, nil
}

// authorizeConfigAccess returns ErrConfigNotFound when the token resolved to
// no config, ErrConfigAccessDenied when it resolved but userID isn't its
// owner, and nil otherwise.
func authorizeConfigAccess(cfg models.FilterConfig, found bool, userID string) error {
	if !found {
		return ErrConfigNotFound
	}
	if cfg.UserID != userID {
		return ErrConfigAccessDenied
	}
	return nil
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
