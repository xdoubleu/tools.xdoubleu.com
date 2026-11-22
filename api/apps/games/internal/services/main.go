package services

import (
	"context"
	"log/slog"

	"tools.xdoubleu.com/apps/games/pkg/steam"
	"tools.xdoubleu.com/apps/internal/progressws"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/jobqueue"
)

type Services struct {
	Auth         auth.Service
	Steam        *SteamService
	Progress     *ProgressService
	Integrations *IntegrationsService
	Library      *LibraryService
	WebSocket    *progressws.Service
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *jobqueue.JobQueue,
	stores Stores,
	steamFactory func(apiKey string) steam.Client,
	authService auth.Service,
) *Services {
	integrations := &IntegrationsService{
		repo: stores.Integrations,
	}

	steamSvc := &SteamService{
		logger:        logger,
		clientFactory: steamFactory,
		steamAPIKey:   config.SteamAPIKey,
		steam:         stores.Steam,
		integrations:  integrations,
	}

	progress := NewProgressService(stores.Progress, steamSvc)

	return &Services{
		Auth:         authService,
		Steam:        steamSvc,
		Progress:     progress,
		Integrations: integrations,
		Library:      NewLibraryService(steamSvc, progress),
		WebSocket: progressws.NewService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}
