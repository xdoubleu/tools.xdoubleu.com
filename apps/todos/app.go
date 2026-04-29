package todos

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/threading"
	"tools.xdoubleu.com/apps/todos/internal/jobs"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
	"tools.xdoubleu.com/apps/todos/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

//go:embed templates/html/**/*.html
var htmlTemplates embed.FS

type Todos struct {
	app.Base
	services *services.Services
	repos    *repositories.Repositories
	jobQueue *threading.JobQueue
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sharedTpl *template.Template,
) *Todos {
	//nolint:exhaustruct //services, repos, jobQueue initialised below
	a := &Todos{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
			htmlTemplates,
			sharedTpl,
		),
	}

	const workers = 1
	const queueSize = 10
	a.jobQueue = threading.NewJobQueue(a.Ctx, logger, workers, queueSize)

	a.repos = repositories.New(db)
	a.services = services.New(a.Logger, a.repos, authService)

	return a
}

func (a *Todos) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Todos) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(jobs.NewArchiveJob(a.repos.Tasks), noop)
}

func (a *Todos) GetName() string {
	return "todos"
}

func (a *Todos) GetDisplayName() string {
	return "Todos"
}
