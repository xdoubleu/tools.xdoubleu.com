package recipes

import (
	"context"
	"embed"
	"html/template"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"tools.xdoubleu.com/apps/recipes/internal/repositories"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

//go:embed templates/html/**/*.html
var htmlTemplates embed.FS

type Recipes struct {
	app.Base
	services *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sharedTpl *template.Template,
) *Recipes {
	//nolint:exhaustruct //services initialised below
	a := &Recipes{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
			htmlTemplates,
			sharedTpl,
		),
	}
	a.services = services.New(a.Logger, repositories.New(db), authService)

	return a
}

func (a *Recipes) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Recipes) Start() error {
	return nil
}

func (a *Recipes) GetName() string {
	return "recipes"
}

func (a *Recipes) GetDisplayName() string {
	return "Recipes"
}
