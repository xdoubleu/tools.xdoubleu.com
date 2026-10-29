// Package learningpaths implements agent-authorable learning curricula: a
// LearningPath of ordered Modules of ordered Items a user checks off, plus a
// freeform resources list.
package learningpaths

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	"tools.xdoubleu.com/apps/learningpaths/internal/todoist"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type LearningPaths struct {
	app.Base
	services *services.Services
}

// New builds the app. sealer encrypts per-user Todoist tokens. books and
// feeds must already be constructed (registration order in cmd/api/apps.go);
// only their exported methods are called via the narrow lookup interfaces
// (docs/adr-0007-dashboard-app-owns-public-sharing.md).
func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sealer *crypto.Sealer,
	books services.BookLookup,
	feeds services.FeedItemLookup,
) *LearningPaths {
	//nolint:exhaustruct //services initialised below
	a := &LearningPaths{
		Base: app.NewBase(
			ctx,
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
		books,
		feeds,
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
