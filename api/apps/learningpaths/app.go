// Package learningpaths implements agent-authorable learning curricula: a
// LearningPath of ordered Modules of ordered Items a user checks off, plus a
// freeform resources list.
package learningpaths

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/todoist"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type LearningPaths struct {
	app.Base
	services *services.Services
}

// New builds the app. sealer encrypts per-user Todoist tokens. booksApp and
// feedsApp must already be constructed (registration order in
// cmd/api/apps.go); only their exported methods are called
// (docs/adr-0007-dashboard-app-owns-public-sharing.md).
func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sealer *crypto.Sealer,
	booksApp *books.Books,
	feedsApp *feeds.Feeds,
) *LearningPaths {
	//nolint:exhaustruct //services initialised below
	a := &LearningPaths{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
		),
	}
	todoistConf := todoist.OAuthConfig(
		cfg.TodoistOAuthClientID, cfg.TodoistOAuthClientSecret, cfg.APIURL,
	)
	a.services = services.New(
		a.Logger,
		repositories.New(db, sealer),
		authService,
		todoistConf,
		booksApp,
		feedsApp,
	)

	return a
}

func (a *LearningPaths) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *LearningPaths) Start() error {
	return nil
}

func (a *LearningPaths) GetName() string {
	return "learningpaths"
}

func (a *LearningPaths) GetDisplayName() string {
	return "Learning Paths"
}

// TodoistServiceForTest exposes the Todoist service to external tests.
func (a *LearningPaths) TodoistServiceForTest() *services.TodoistService {
	return a.services.Todoist
}
