//nolint:revive //it is what it is
package goaltracker

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	_ "time/tzdata"

	"github.com/XDoubleU/essentia/pkg/database/postgres"
	"github.com/XDoubleU/essentia/pkg/threading"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"tools.xdoubleu.com/apps/goaltracker/internal/jobs"
	"tools.xdoubleu.com/apps/goaltracker/internal/repositories"
	"tools.xdoubleu.com/apps/goaltracker/internal/services"
	"tools.xdoubleu.com/apps/goaltracker/pkg/goodreads"
	"tools.xdoubleu.com/apps/goaltracker/pkg/steam"
	"tools.xdoubleu.com/apps/goaltracker/pkg/todoist"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

//go:embed images/**
var images embed.FS

type GoalTracker struct {
	logger       *slog.Logger
	ctx          context.Context
	ctxCancel    context.CancelFunc
	db           postgres.DB
	Config       config.Config
	images       embed.FS
	clients      Clients
	Services     *services.Services
	Repositories *repositories.Repositories
	tpl          *template.Template
	jobQueue     *threading.JobQueue
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *GoalTracker {
	clients := Clients{
		Todoist:   todoist.New(cfg.TodoistAPIKey),
		Steam:     steam.New(logger, cfg.SteamAPIKey),
		Goodreads: goodreads.New(logger),
	}

	return NewInner(authService, logger, cfg, db, clients)
}

func NewInner(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	clients Clients,
) *GoalTracker {
	tpl := template.Must(template.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	//nolint:mnd //no magic number
	jobQueue := threading.NewJobQueue(logger, 2, 100)

	//nolint:exhaustruct //other fields are optional
	app := &GoalTracker{
		logger:   logger,
		clients:  clients,
		Config:   cfg,
		images:   images,
		tpl:      tpl,
		jobQueue: jobQueue,
	}

	app.setContext()
	app.setDB(db, authService)
	app.setJobs()

	return app
}

func (app *GoalTracker) setDB(
	db postgres.DB,
	authService auth.Service,
) {
	// make sure previous app is cancelled internally
	app.ctxCancel()
	app.jobQueue.Clear()

	app.setContext()

	spandb := postgres.NewSpanDB(db)
	app.db = spandb

	app.Repositories = repositories.New(app.db)
	app.Services = services.New(
		app.logger,
		app.Config,
		app.jobQueue,
		app.Repositories,
		app.clients.Todoist,
		app.clients.Steam,
		app.clients.Goodreads,
		authService,
	)
}

func (app *GoalTracker) setJobs() {
	err := app.jobQueue.AddJob(
		jobs.NewTodoistJob(app.Services.Auth, app.Services.Goals),
		app.Services.WebSocket.UpdateState,
	)
	if err != nil {
		panic(err)
	}

	err = app.jobQueue.AddJob(
		jobs.NewGoodreadsJob(
			app.Services.Auth,
			app.Services.Goodreads,
			app.Services.Goals,
		),
		app.Services.WebSocket.UpdateState,
	)
	if err != nil {
		panic(err)
	}

	err = app.jobQueue.AddJob(
		jobs.NewSteamJob(app.Services.Auth, app.Services.Steam, app.Services.Goals),
		app.Services.WebSocket.UpdateState,
	)
	if err != nil {
		panic(err)
	}

	app.Services.WebSocket.RegisterTopics(app.jobQueue.FetchJobIDs())
}

func (app *GoalTracker) setContext() {
	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	app.ctxCancel = cancel
}

func (app *GoalTracker) ApplyMigrations(db *pgxpool.Pool) error {
	migrationsDB := stdlib.OpenDBFromPool(db)

	goose.SetLogger(slog.NewLogLogger(app.logger.Handler(), slog.LevelInfo))

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect(string(goose.DialectPostgres)); err != nil {
		return err
	}

	if err := goose.Up(migrationsDB, "migrations"); err != nil {
		return err
	}

	return nil
}

func (app *GoalTracker) GetName() string {
	return "goaltracker"
}
