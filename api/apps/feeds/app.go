package feeds

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/feeds/internal/jobs"
	"tools.xdoubleu.com/apps/feeds/internal/repositories"
	"tools.xdoubleu.com/apps/feeds/internal/services"
	"tools.xdoubleu.com/apps/feeds/pkg/webfetch"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/observability"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Feeds struct {
	app.Base
	db postgres.DB
	// Services is exported so integration tests can seed data through the
	// real service layer.
	Services    *services.Services
	jobQueue    *threading.JobQueue
	feedPollJob *jobs.FeedPollJob
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Feeds {
	return NewInner(authService, logger, cfg, db, webfetch.New(logger))
}

// NewInner allows tests to inject a fake webfetch.Client.
func NewInner(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	webFetchClient webfetch.Client,
) *Feeds {
	//nolint:exhaustruct // jobQueue/Services/feedPollJob initialised below
	a := &Feeds{
		Base: app.NewBase(context.Background(), authService, logger, cfg),
		db:   db,
	}

	const amountOfWorkers = 1
	const jobQueueSize = 10
	a.jobQueue = threading.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize)

	repos := repositories.New(db)
	a.Services = services.New(
		logger,
		repos,
		webFetchClient,
		cfg.EmailInboundDomain,
		authService,
	)
	a.feedPollJob = jobs.NewFeedPollJob(a.Services.Feeds)

	return a
}

func (a *Feeds) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(
		observability.NewTrackedJob(a.feedPollJob, a.db),
		noop,
	)
}

func (a *Feeds) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

// RunPollNow runs the RSS poll synchronously, wrapped in the same TrackedJob
// used for the scheduled run so a manual trigger still shows up in
// global.job_runs.
func (a *Feeds) RunPollNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.feedPollJob, a.db).Run(ctx, a.Logger)
}

func (a *Feeds) GetName() string {
	return "feeds"
}

func (a *Feeds) GetDisplayName() string {
	return "Feeds"
}
