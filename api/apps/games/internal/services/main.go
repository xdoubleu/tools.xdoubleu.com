package services

import (
	"context"
	"log/slog"

	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/games/internal/repositories"
	"tools.xdoubleu.com/apps/games/pkg/steam"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/progressws"
)

type Services struct {
	Auth         auth.Service
	Steam        *SteamService
	Progress     *ProgressService
	Integrations *IntegrationsService
	WebSocket    *progressws.Service
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *threading.JobQueue,
	repositories *repositories.Repositories,
	steamFactory func(apiKey string) steam.Client,
	authService auth.Service,
) *Services {
	integrations := &IntegrationsService{
		repo: repositories.Integrations,
	}

	steamSvc := &SteamService{
		logger:        logger,
		clientFactory: steamFactory,
		steamAPIKey:   config.SteamAPIKey,
		steam:         repositories.Steam,
		progress:      repositories.Progress,
		integrations:  integrations,
	}

	return &Services{
		Auth:         authService,
		Steam:        steamSvc,
		Progress:     NewProgressService(repositories.Progress, steamSvc),
		Integrations: integrations,
		WebSocket: progressws.NewService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}
