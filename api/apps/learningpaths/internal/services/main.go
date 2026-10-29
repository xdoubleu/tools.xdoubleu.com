package services

import (
	"log/slog"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/oauthconn"
)

type Services struct {
	Auth          auth.Service
	LearningPaths *LearningPathService
	Todoist       *TodoistService
}

// New wires the services; books/feeds are used only via their exported
// lookup methods.
func New(
	logger *slog.Logger,
	repos *repositories.Repositories,
	authService auth.Service,
	todoistConf *oauth2.Config,
	books BookLookup,
	feeds FeedItemLookup,
) *Services {
	todoist := NewTodoistService(
		repos.OAuthConnections, repos.LearningPaths, todoistConf,
		oauthconn.NewStateStore(),
	)
	learningPaths := &LearningPathService{
		logger:  logger,
		repo:    repos.LearningPaths,
		books:   books,
		feeds:   feeds,
		todoist: todoist,
	}
	todoist.resolveItems = learningPaths.resolveItemLinks
	return &Services{
		Auth:          authService,
		LearningPaths: learningPaths,
		Todoist:       todoist,
	}
}
