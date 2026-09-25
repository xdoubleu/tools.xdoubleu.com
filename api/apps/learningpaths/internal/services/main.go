package services

import (
	"log/slog"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/oauthconn"
)

type Services struct {
	Auth          auth.Service
	LearningPaths *LearningPathService
	Todoist       *TodoistService
}

// New wires the services; booksApp/feedsApp are used only via their exported
// lookup methods.
func New(
	_ *slog.Logger,
	repos *repositories.Repositories,
	authService auth.Service,
	todoistConf *oauth2.Config,
	booksApp *books.Books,
	feedsApp *feeds.Feeds,
) *Services {
	learningPaths := &LearningPathService{
		repo:  repos.LearningPaths,
		books: booksApp,
		feeds: feedsApp,
	}
	return &Services{
		Auth:          authService,
		LearningPaths: learningPaths,
		Todoist: NewTodoistService(
			repos.OAuthConnections, repos.LearningPaths, todoistConf,
			oauthconn.NewStateStore(),
		),
	}
}
