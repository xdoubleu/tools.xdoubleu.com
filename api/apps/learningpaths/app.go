// Package learningpaths implements agent-authorable learning curricula: a
// LearningPath (goal, recurring routine) made of ordered Modules, each with
// ordered Items a user checks off as they progress, plus a freeform
// resources list. This slice (#1472) is the core app — proto, schema,
// ConnectRPC CRUD, and a minimal web UI. MCP tools (#1473), books/feeds
// resource linking (#1474), and Todoist integration (#1475) are later,
// independent slices stacked on top of this one.
package learningpaths

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type LearningPaths struct {
	app.Base
	services *services.Services
}

// New scopes every row by user_id alone (no FamilyRepository, no
// family_id) — an explicit opt-out of ADR-0008's family-sharing model for
// this app, matching games/internal/repositories/progress.go's pattern.
func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
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
	a.services = services.New(a.Logger, repositories.New(db), authService)

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
