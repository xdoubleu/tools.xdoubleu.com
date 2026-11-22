package services

import (
	"log/slog"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/oauthconn"
)

type Services struct {
	Auth          auth.Service
	LearningPaths *LearningPathService
	Todoist       *TodoistService
}

// New wires the services over the given storage and the books/feeds ports.
func New(
	logger *slog.Logger,
	store LearningPathsStore,
	connections TodoistConnections,
	authService auth.Service,
	todoistConf *oauth2.Config,
	books BookLookup,
	feeds FeedItemLookup,
) *Services {
	todoist := NewTodoistService(
		connections, store, todoistConf,
		oauthconn.NewStateStore(),
	)
	learningPaths := &LearningPathService{
		logger:  logger,
		repo:    store,
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
