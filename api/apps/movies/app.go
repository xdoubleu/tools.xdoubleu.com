// Package movies implements a per-user movies & series backlog backed by
// TMDB metadata.
package movies

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/movies/internal/jobs"
	"tools.xdoubleu.com/apps/movies/internal/repositories"
	"tools.xdoubleu.com/apps/movies/internal/services"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/observability"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Movies struct {
	app.Base
	db         postgres.DB
	services   *services.Services
	jobQueue   *jobqueue.JobQueue
	refreshJob jobs.RefreshMetadataJob
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Movies {
	return NewWithTMDB(authService, logger, cfg, db, newTMDBClient(logger, cfg))
}

// NewWithTMDB builds the app around the given TMDB client; nil disables
// search and adding.
func NewWithTMDB(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	tmdbClient tmdb.Client,
) *Movies {
	//nolint:exhaustruct //services, jobs initialised below
	a := &Movies{
		Base: app.NewBase(context.Background(), authService, logger, cfg),
		db:   db,
	}
	a.services = services.New(repositories.New(db), authService, tmdbClient)

	const amountOfWorkers = 1
	const jobQueueSize = 10
	a.jobQueue = jobqueue.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize, db)
	a.refreshJob = jobs.NewRefreshMetadataJob(a.services.Movies)
	return a
}

func newTMDBClient(logger *slog.Logger, cfg config.Config) tmdb.Client {
	if cfg.TMDBAPIKey == "" {
		logger.Warn("TMDB_API_KEY not set; movies search is disabled")
		return nil
	}
	return tmdb.New(cfg.TMDBAPIKey)
}

func (a *Movies) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Movies) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(observability.NewTrackedJob(a.refreshJob, a.db), noop)
}

// RefreshNow runs the catalog refresh job once, outside its schedule.
func (a *Movies) RefreshNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.refreshJob, a.db).Run(ctx, a.Logger)
}

func (a *Movies) GetName() string {
	return "movies"
}

func (a *Movies) GetDisplayName() string {
	return "Movies & Series"
}
