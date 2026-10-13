// Package taste builds a per-user consumption log across every media app, its
// cross-domain statistics, and an LLM-written taste profile. It reaches other
// apps only through their exported ListConsumption methods.
package taste

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/repositories"
	"tools.xdoubleu.com/apps/taste/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/observability"
	"tools.xdoubleu.com/internal/openrouter"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Source is an app exporting its consumed items.
type Source = services.Source

// Catalogs look recommendation candidates up in each domain's catalog.
type Catalogs = services.Catalogs

type Taste struct {
	app.Base
	db       postgres.DB
	auth     auth.Service
	log      *services.LogService
	profiles *services.ProfileService
	picks    *services.RecommendService
	jobQueue *jobqueue.JobQueue
	genJob   jobs.GenerateProfilesJob
}

// New builds the app over sources, which must already be constructed
// (registration order in cmd/api/apps.go).
func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	catalogs Catalogs,
	sources ...Source,
) *Taste {
	llm := openrouter.New(cfg.OpenRouterAPIKey, cfg.WebURL, logger)
	return NewInner(authService, logger, cfg, db, llm, catalogs, sources...)
}

// NewInner allows tests to inject a fake OpenRouter client; it counts as
// configured when cfg has a key.
func NewInner(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	llm openrouter.Client,
	catalogs Catalogs,
	sources ...Source,
) *Taste {
	if cfg.OpenRouterAPIKey == "" {
		logger.Warn("OPENROUTER_API_KEY not set; taste profiles are disabled")
	}
	//nolint:exhaustruct // jobQueue, genJob initialised below
	a := &Taste{
		Base: app.NewBase(context.Background(), authService, logger, cfg),
		db:   db,
		auth: authService,
		log:  services.NewLogService(sources),
	}
	profiles := repositories.NewProfilesRepository(db)
	a.profiles = services.NewProfileService(
		a.log, profiles, llm, cfg.TasteGeneratorModel, cfg.OpenRouterAPIKey != "",
	)
	a.picks = services.NewRecommendService(
		a.log, profiles, repositories.NewRecommendationsRepository(db), catalogs,
		llm, cfg.TasteGeneratorModel, logger,
	)

	const amountOfWorkers = 1
	const jobQueueSize = 10
	a.jobQueue = jobqueue.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize, db)
	a.genJob = jobs.NewGenerateProfilesJob(
		authService, a.profiles, a.picks, a.GetName(),
	)
	return a
}

func (a *Taste) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Taste) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(observability.NewTrackedJob(a.genJob, a.db), noop)
}

// GenerateProfilesNow runs the profile job once, outside its schedule.
func (a *Taste) GenerateProfilesNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.genJob, a.db).Run(ctx, a.Logger)
}

func (a *Taste) GetName() string {
	return "taste"
}

func (a *Taste) GetDisplayName() string {
	return "Taste"
}
