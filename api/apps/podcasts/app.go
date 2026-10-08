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
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Podcasts {
	return NewWithITunes(authService, logger, cfg, db, itunes.New())
}

// NewWithITunes builds the app around the given iTunes client.
func NewWithITunes(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	itunesClient itunes.Client,
) *Podcasts {
	return &Podcasts{
		Base:     app.NewBase(context.Background(), authService, logger, cfg),
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

// Start has nothing to run yet; episode polling arrives with the next slice.
func (a *Podcasts) Start() error {
	return nil
}
