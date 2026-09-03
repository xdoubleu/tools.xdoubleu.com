// Package trains ingests the SNCB/NMBS timetable and (in later slices, see
// issue #1388) computes journeys and overlays realtime delays. This slice
// (#1390) is the app shell plus the daily GTFS static feed import; it has
// no user-visible half of its own.
package trains

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
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
	Services        *services.Services
	Repositories    *repositories.Repositories
	jobQueue        *jobqueue.JobQueue
	staticImportJob *jobs.StaticImportJob
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

	return a
}

func (a *Trains) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(
		observability.NewTrackedJob(a.staticImportJob, a.db),
		noop,
	)
}

// Routes registers no HTTP routes yet — this slice has no user-visible half
// (issue #1390). Slice 4 (#1392) adds the ConnectRPC service.
func (a *Trains) Routes(_ string, _ *http.ServeMux) {}

func (a *Trains) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Trains) GetName() string { return "trains" }

func (a *Trains) GetDisplayName() string { return "Trains" }

func (a *Trains) GetDomain() string { return "" }
