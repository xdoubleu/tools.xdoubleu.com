package services

import (
	"log/slog"

	"github.com/xdoubleu/essentia/v2/pkg/threading"
	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
	"tools.xdoubleu.com/apps/goaltracker/pkg/steam"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

type Services struct {
	Auth      auth.Service
	Goals     *GoalService
	Todoist   *TodoistService
	Steam     *SteamService
	Goodreads *GoodreadsService
	WebSocket *WebSocketService
}

func New(
	logger *slog.Logger,
	config config.Config,
	jobQueue *threading.JobQueue,
	repositories *repositories.Repositories,
	todoistClient todoist.Client,
	steamClient steam.Client,
	goodreadsClient goodreads.Client,
	authService auth.Service,
) *Services {
	goodreads := &GoodreadsService{
		logger:     logger,
		profileURL: config.GoodreadsURL,
		goodreads:  repositories.Goodreads,
		client:     goodreadsClient,
	}
	todoist := &TodoistService{
		client:    todoistClient,
		projectID: config.TodoistProjectID,
	}
	steam := &SteamService{
		logger: logger,
		client: steamClient,
		userID: config.SteamUserID,
		steam:  repositories.Steam,
	}
	goals := &GoalService{
		webURL:    config.WebURL,
		states:    repositories.States,
		goals:     repositories.Goals,
		progress:  repositories.Progress,
		todoist:   todoist,
		goodreads: goodreads,
		steam:     steam,
	}

	return &Services{
		Auth:      authService,
		Goals:     goals,
		Todoist:   todoist,
		Steam:     steam,
		Goodreads: goodreads,
		WebSocket: NewWebSocketService(logger, []string{config.WebURL}, jobQueue),
	}
}
