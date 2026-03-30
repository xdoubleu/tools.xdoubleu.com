//nolint:revive //it is what it is
package goaltracker

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v3/pkg/threading"
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

type GoalTracker struct {
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
) *GoalTracker {
	clients := Clients{
		Todoist:   todoist.New(cfg.TodoistAPIKey),
		Steam:     steam.New(logger, cfg.SteamAPIKey),
		Goodreads: goodreads.New(logger),
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
) *GoalTracker {
	tpl := template.Must(sharedTpl.Clone())
	tpl = template.Must(tpl.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	//nolint:exhaustruct //other fields are optional
	app := &GoalTracker{
		logger:  logger,
		clients: clients,
		Config:  cfg,
		tpl:     tpl,
	}

	app.setContext(ctx)

	//nolint:mnd //no magic number
	app.jobQueue = threading.NewJobQueue(app.ctx, logger, 2, 100)

	app.setDB(db, authService)
	app.setJobs()

	return app
}

func (app *GoalTracker) setDB(
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

func (app *GoalTracker) setContext(originalCtx context.Context) {
	if app.ctxCancel != nil {
		app.ctxCancel()
	}

	ctx, cancel := context.WithCancel(originalCtx)
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
