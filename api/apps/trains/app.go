// Package trains ingests the SNCB/NMBS timetable and (in later slices, see
// issue #1388) overlays realtime delays. #1390 added the daily GTFS static
// import; this slice (#1391) adds trains.v1.TrainService.SearchJourneys, a
// Connection Scan Algorithm journey planner over an in-memory index built
// from a rolling window of the ingested timetable.
package trains

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/trains/internal/jobs"
	"tools.xdoubleu.com/apps/trains/internal/repositories"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/bmc"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/observability"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Trains struct {
	app.Base
	db postgres.DB
	// Services and Repositories are exported so integration tests can drive
	// the real import path (same convention as games/books).
	Services         *services.Services
	Repositories     *repositories.Repositories
	jobQueue         *jobqueue.JobQueue
	staticImportJob  *jobs.StaticImportJob
	routerRefreshJob *jobs.RouterRefreshJob
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Trains {
	if cfg.BMCPartnerKey == "" {
		logger.Warn(
			"BMC_PARTNER_KEY is not set — SNCB timetable import will be disabled",
		)
	}
	return NewInner(
		authService, logger, cfg, db,
		bmc.New(logger, cfg.BMCHost, cfg.BMCPartnerKey),
	)
}

// NewInner lets tests inject a fake bmc.Client.
func NewInner(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	bmcClient bmc.Client,
) *Trains {
	//nolint:exhaustruct //jobQueue/Services/Repositories initialised below
	a := &Trains{
		Base: app.NewBase(context.Background(), authService, logger, cfg),
		db:   db,
	}

	const amountOfWorkers = 1
	const jobQueueSize = 10
	a.jobQueue = jobqueue.NewJobQueue(
		a.Ctx, logger, amountOfWorkers, jobQueueSize, a.db,
	)

	a.Repositories = repositories.New(a.db)
	a.Services = services.New(logger, a.Repositories, bmcClient)
	a.staticImportJob = jobs.NewStaticImportJob(a.Services.StaticImport)
	a.routerRefreshJob = jobs.NewRouterRefreshJob(a.Services.Journey.RefreshOnly)

	return a
}

func (a *Trains) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	if err := a.jobQueue.AddJob(
		observability.NewTrackedJob(a.staticImportJob, a.db),
		noop,
	); err != nil {
		return err
	}
	return a.jobQueue.AddJob(
		observability.NewTrackedJob(a.routerRefreshJob, a.db),
		noop,
	)
}

func (a *Trains) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Trains) GetName() string { return "trains" }

func (a *Trains) GetDisplayName() string { return "Trains" }

func (a *Trains) GetDomain() string { return "" }
