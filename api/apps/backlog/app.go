package backlog

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/backlog/internal/jobs"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/objectstore"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Backlog struct {
	app.Base
	db           postgres.DB
	clients      Clients
	Services     *services.Services
	Repositories *repositories.Repositories
	jobQueue     *threading.JobQueue
}

func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Backlog {
	if cfg.R2AccountID == "" || cfg.R2AccessKeyID == "" ||
		cfg.R2SecretKey == "" || cfg.R2Bucket == "" {
		logger.Warn(
			"R2 object store is not fully configured — book file uploads will fail;" +
				" set R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET",
		)
	}

	if cfg.HardcoverAPIKey == "" {
		logger.Warn(
			"HARDCOVER_API_KEY is not set — Hardcover book search will be disabled",
		)
	}

	if cfg.SteamAPIKey == "" {
		logger.Warn(
			"STEAM_API_KEY is not set — Steam sync will be disabled",
		)
	}

	endpoint := "https://" + cfg.R2AccountID + ".r2.cloudflarestorage.com"

	clients := Clients{
		SteamFactory: func(apiKey string) steam.Client {
			return steam.New(logger, apiKey)
		},
		HardcoverFactory: func(apiKey string) hardcover.Client {
			return hardcover.New(logger, apiKey)
		},
		ObjectStore: objectstore.NewR2(
			endpoint,
			cfg.R2AccessKeyID,
			cfg.R2SecretKey,
			cfg.R2Bucket,
		),
		KoboStoreBaseURL: "https://storeapi.kobo.com",
	}

	return NewInner(ctx, authService, logger, cfg, db, clients)
}

func NewInner(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	clients Clients,
) *Backlog {
	//nolint:exhaustruct //jobQueue, Repositories, Services initialised below
	bl := &Backlog{
		Base:    app.NewBase(ctx, authService, logger, cfg),
		clients: clients,
	}

	const amountOfWorkers = 2
	const jobQueueSize = 100
	bl.jobQueue = threading.NewJobQueue(bl.Ctx, logger, amountOfWorkers, jobQueueSize)

	bl.setDB(db, authService)

	return bl
}

func (app *Backlog) Start() error {
	app.setJobs()
	return nil
}

func (app *Backlog) setDB(
	db postgres.DB,
	authService auth.Service,
) {
	spandb := postgres.NewSpanDB(db)
	app.db = spandb

	app.Repositories = repositories.New(app.db)
	app.Services = services.New(
		app.Ctx,
		app.Logger,
		app.Config,
		app.jobQueue,
		app.Repositories,
		app.clients.SteamFactory,
		app.clients.HardcoverFactory,
		app.clients.ObjectStore,
		authService,
	)
}

func (app *Backlog) setJobs() {
	err := app.jobQueue.AddJob(
		jobs.NewSteamJob(app.Services.Auth, app.Services.Steam),
		app.Services.WebSocket.UpdateState,
	)
	if err != nil {
		panic(err)
	}

	app.Services.WebSocket.RegisterTopics(app.jobQueue.FetchJobIDs())
}

func (app *Backlog) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return app.ApplyMigrationsFromFS(ctx, db, embedMigrations, app.GetName())
}

func (app *Backlog) GetName() string {
	return "backlog"
}

func (app *Backlog) GetDisplayName() string {
	return "Backlog"
}
