package backlog

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v3/pkg/threading"
	"tools.xdoubleu.com/apps/backlog/internal/jobs"
	"tools.xdoubleu.com/apps/backlog/internal/repositories"
	"tools.xdoubleu.com/apps/backlog/internal/services"
	"tools.xdoubleu.com/apps/backlog/pkg/hardcover"
	"tools.xdoubleu.com/apps/backlog/pkg/steam"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

type Backlog struct {
	logger       *slog.Logger
	ctx          context.Context
	ctxCancel    context.CancelFunc
	db           postgres.DB
	Config       config.Config
	clients      Clients
	Services     *services.Services
	Repositories *repositories.Repositories
	tpl          *template.Template
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
	tpl := template.Must(sharedTpl.Clone())
	tpl = template.Must(tpl.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	//nolint:exhaustruct //other fields are optional
	app := &Backlog{
		logger:  logger,
		clients: clients,
		Config:  cfg,
		tpl:     tpl,
	}

	app.setContext(ctx)

	const amountOfWorkers = 2
	const jobQueueSize = 100
	app.jobQueue = threading.NewJobQueue(app.ctx, logger, amountOfWorkers, jobQueueSize)

	app.setDB(db, authService)

	return app
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
		app.ctx,
		app.logger,
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

func (app *Backlog) setContext(originalCtx context.Context) {
	if app.ctxCancel != nil {
		app.ctxCancel()
	}

	ctx, cancel := context.WithCancel(originalCtx)
	app.ctx = ctx
	app.ctxCancel = cancel
}

func (app *Backlog) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	schemaName := "backlog"

	_, err := db.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName))
	if err != nil {
		return err
	}

	goose.SetTableName(fmt.Sprintf("%s.goose_db_version", schemaName))
	goose.SetLogger(slog.NewLogLogger(app.logger.Handler(), slog.LevelInfo))
	goose.SetBaseFS(embedMigrations)

	if err = goose.SetDialect(string(goose.DialectPostgres)); err != nil {
		return err
	}

	migrationsDB := stdlib.OpenDBFromPool(db)
	if err = goose.Up(migrationsDB, "migrations"); err != nil {
		return err
	}

	return nil
}

func (app *Backlog) GetName() string {
	return "backlog"
}
