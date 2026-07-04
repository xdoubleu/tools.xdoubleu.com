package games

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/games/internal/jobs"
	"tools.xdoubleu.com/apps/games/internal/repositories"
	"tools.xdoubleu.com/apps/games/internal/services"
	"tools.xdoubleu.com/apps/games/pkg/steam"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Games struct {
	app.Base
	db postgres.DB
	// Services and Repositories are exported so integration tests can seed
	// data through the real service layer.
	Services     *services.Services
	Repositories *repositories.Repositories
	jobQueue     *threading.JobQueue
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Games {
	if cfg.SteamAPIKey == "" {
		logger.Warn(
			"STEAM_API_KEY is not set — Steam sync will be disabled",
		)
	}

	steamFactory := func(apiKey string) steam.Client {
		return steam.New(logger, apiKey)
	}

	return NewInner(authService, logger, cfg, db, steamFactory)
}

func NewInner(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	steamFactory func(apiKey string) steam.Client,
) *Games {
	//nolint:exhaustruct //jobQueue, Repositories, Services initialised below
	a := &Games{
		Base: app.NewBase(context.Background(), authService, logger, cfg),
		db:   db,
	}

	const amountOfWorkers = 1
	const jobQueueSize = 100
	a.jobQueue = threading.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize)

	a.Repositories = repositories.New(a.db)
	a.Services = services.New(
		a.Ctx,
		logger,
		a.Config,
		a.jobQueue,
		a.Repositories,
		steamFactory,
		authService,
	)

	return a
}

func (a *Games) Start() error {
	if err := a.jobQueue.AddJob(
		jobs.NewSteamJob(a.Services.Auth, a.Services.Steam),
		a.Services.WebSocket.UpdateState,
	); err != nil {
		return err
	}

	a.Services.WebSocket.RegisterTopics(a.jobQueue.FetchJobIDs())
	return nil
}

func (a *Games) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Games) GetName() string {
	return "games"
}

func (a *Games) GetDisplayName() string {
	return "Games"
}
