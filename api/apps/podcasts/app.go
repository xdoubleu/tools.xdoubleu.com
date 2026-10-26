// Package podcasts implements per-user favourite podcast shows found through
// the iTunes Search API.
package podcasts

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/podcasts/internal/repositories"
	"tools.xdoubleu.com/apps/podcasts/internal/services"
	"tools.xdoubleu.com/apps/podcasts/pkg/itunes"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Podcasts struct {
	app.Base
	services *services.Services
}

func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Podcasts {
	return NewInner(ctx, authService, logger, cfg, db, itunes.New())
}

// NewInner allows tests to inject a fake iTunes client.
func NewInner(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	itunesClient itunes.Client,
) *Podcasts {
	return &Podcasts{
		Base:     app.NewBase(ctx, authService, logger, cfg),
		services: services.New(repositories.New(db), authService, itunesClient),
	}
}

func (a *Podcasts) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Podcasts) GetName() string {
	return "podcasts"
}

func (a *Podcasts) GetDisplayName() string {
	return "Podcasts"
}

func (a *Podcasts) Start() error {
	return nil
}
