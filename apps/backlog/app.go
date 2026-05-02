package backlog

import (
	"context"
	"embed"
	"html/template"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/threading"
	"tools.xdoubleu.com/apps/backlog/internal/jobs"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

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
	sharedTpl *template.Template,
) *Backlog {
	clients := Clients{
		SteamFactory: func(apiKey string) steam.Client {
			return steam.New(logger, apiKey)
		},
		HardcoverFactory: func(apiKey string) hardcover.Client {
			return hardcover.New(logger, apiKey)
		},
	}

	return NewInner(ctx, authService, logger, cfg, db, clients, sharedTpl)
}

func NewInner(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	clients Clients,
	sharedTpl *template.Template,
) *Backlog {
	//nolint:exhaustruct //jobQueue, Repositories, Services initialised below
	bl := &Backlog{
		Base:    app.NewBase(ctx, authService, logger, cfg, htmlTemplates, sharedTpl),
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

	app.Repositories = repositories.New(app.db, app.Config.EncryptionKey)
	app.Services = services.New(
		app.Ctx,
		app.Logger,
		app.Config,
		app.jobQueue,
		app.Repositories,
		app.clients.SteamFactory,
		app.clients.HardcoverFactory,
		authService,
	)
}

func (app *Backlog) setJobs() {
	err := app.jobQueue.AddJob(
		jobs.NewSteamJob(app.Services.Auth, app.Services.Steam, app.Services.Progress),
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
