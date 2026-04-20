package services

import (
	"context"
	"log/slog"

	"github.com/xdoubleu/essentia/v3/pkg/threading"
	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
	"tools.xdoubleu.com/apps/goaltracker/pkg/steam"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

type Services struct {
	Auth         auth.Service
	Goals        *GoalService
	Todoist      *TodoistService
	Steam        *SteamService
	Goodreads    *GoodreadsService
	Integrations *IntegrationsService
	WebSocket    *WebSocketService
}

func New(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	jobQueue *threading.JobQueue,
	repositories *repositories.Repositories,
	todoistFactory func(apiKey string) todoist.Client,
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
	todoistSvc := &TodoistService{
		clientFactory: todoistFactory,
	}
	steamSvc := &SteamService{
		logger:        logger,
		clientFactory: steamFactory,
		steam:         repositories.Steam,
		integrations:  integrations,
	}
	goals := &GoalService{
		webURL:       config.WebURL,
		states:       repositories.States,
		goals:        repositories.Goals,
		progress:     repositories.Progress,
		todoist:      todoistSvc,
		goodreads:    goodreadsSvc,
		steam:        steamSvc,
		integrations: integrations,
	}

	return &Services{
		Auth:         authService,
		Goals:        goals,
		Todoist:      todoistSvc,
		Steam:        steamSvc,
		Goodreads:    goodreadsSvc,
		Integrations: integrations,
		WebSocket: NewWebSocketService(
			ctx,
			logger,
			[]string{config.WebURL},
			jobQueue,
		),
	}
}
