package services

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/icsproxy/internal/models"
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
)

var (
	ErrConfigNotFound     = errors.New("config not found")
	ErrConfigAccessDenied = errors.New("permission denied")
)

// calendarStore is the storage surface CalendarService needs. It is
// satisfied by repositories.CalendarRepository and by fakes in unit tests,
// so ownership/error-propagation rules can be tested without a database.
type calendarStore interface {
	UpsertFilterConfig(ctx context.Context, cfg models.FilterConfig) error
	GetFilterConfig(ctx context.Context, token string) (models.FilterConfig, bool)
	ListFilterConfigs(
		ctx context.Context,
		userID string,
		limit, offset int32,
	) ([]models.FilterConfig, bool, error)
	ListFilterSummaries(
		ctx context.Context,
		userID string,
	) ([]repositories.FilterSummary, error)
	DeleteFilterConfig(ctx context.Context, token string, userID string) error
}

type CalendarService struct {
	logger *slog.Logger
	repo   calendarStore
	client *http.Client
}

// SaveConfig upserts cfg and returns the token the feed is served under.
//
// An empty cfg.Token mints a fresh UUID; a non-empty one must already exist
// and be owned by cfg.UserID. Tokens are never accepted from the client for a
// *new* feed: the feed endpoint is unauthenticated, so an unguessable
// server-generated token is the only thing protecting a feed's contents.
func (s *CalendarService) SaveConfig(
	ctx context.Context,
	cfg models.FilterConfig,
) (string, error) {
	if cfg.Token == "" {
		cfg.Token = uuid.NewString()
	} else {
		existing, ok := s.LoadConfig(ctx, cfg.Token)
		if err := authorizeConfigAccess(existing, ok, cfg.UserID); err != nil {
			return "", err
		}
	}

	if err := s.repo.UpsertFilterConfig(ctx, cfg); err != nil {
		return "", err
	}

	return cfg.Token, nil
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
