package services

import (
	"context"
	"log/slog"

	"github.com/xdoubleu/essentia/v3/pkg/threading"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/pkg/goodreads"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

type Services struct {
	Auth         auth.Service
	Steam        *SteamService
	Goodreads    *GoodreadsService
	Progress     *ProgressService
	Backlog      *BacklogService
	Integrations *IntegrationsService
	WebSocket    *WebSocketService
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *threading.JobQueue,
	repositories *repositories.Repositories,
	steamFactory func(apiKey string) steam.Client,
	goodreadsClient goodreads.Client,
	authService auth.Service,
) *Services {
	integrations := &IntegrationsService{
		repo: repositories.Integrations,
	}

	goodreadsSvc := &GoodreadsService{
		logger:       logger,
		goodreads:    repositories.Goodreads,
		client:       goodreadsClient,
		integrations: integrations,
	}
	steamSvc := &SteamService{
		logger:        logger,
		clientFactory: steamFactory,
		steam:         repositories.Steam,
		integrations:  integrations,
	}
	progressSvc := &ProgressService{
		progress: repositories.Progress,
		steam:    steamSvc,
	}
	backlogSvc := &BacklogService{
		steam:     steamSvc,
		goodreads: goodreadsSvc,
	}

	return &Services{
		Auth:         authService,
		Steam:        steamSvc,
		Goodreads:    goodreadsSvc,
		Progress:     progressSvc,
		Backlog:      backlogSvc,
		Integrations: integrations,
		WebSocket: NewWebSocketService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}
