// Package podcasts implements per-user favourite podcast shows found through
// the iTunes Search API.
package podcasts

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	"tools.xdoubleu.com/apps/podcasts/internal/jobs"
	"tools.xdoubleu.com/apps/podcasts/internal/repositories"
	"tools.xdoubleu.com/apps/podcasts/internal/services"
	"tools.xdoubleu.com/apps/podcasts/pkg/itunes"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/observability"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Podcasts struct {
	app.Base
	db       postgres.DB
	services *services.Services
	jobQueue *jobqueue.JobQueue
	pollJob  jobs.PollFeedsJob
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Podcasts {
	return NewInner(
		authService, logger, cfg, db,
		itunes.New(), webfetch.New(logger, AllowPrivateFeeds(cfg.Env)),
	)
}

// AllowPrivateFeeds reports whether feed fetches may reach private addresses,
// which only production forbids.
func AllowPrivateFeeds(env string) bool {
	return env != config.ProdEnv
}

// NewInner allows tests to inject fake iTunes and feed clients.
func NewInner(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	itunesClient itunes.Client,
	fetcher webfetch.Client,
) *Podcasts {
	//nolint:exhaustruct // services, jobQueue, pollJob initialised below
	a := &Podcasts{
		Base: app.NewBase(context.Background(), authService, logger, cfg),
		db:   db,
	}
	a.services = services.New(
		repositories.New(db), authService, itunesClient, fetcher, logger,
	)

	const amountOfWorkers = 1
	const jobQueueSize = 10
	a.jobQueue = jobqueue.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize, db)
	a.pollJob = jobs.NewPollFeedsJob(a.services.Shows)
	return a
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
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(observability.NewTrackedJob(a.pollJob, a.db), noop)
}

// PollNow polls every favourite's feed once, outside the job's schedule.
func (a *Podcasts) PollNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.pollJob, a.db).Run(ctx, a.Logger)
}
