// Package learningpaths implements agent-authorable learning curricula: a
// LearningPath of ordered Modules of ordered Items a user checks off, plus a
// freeform resources list.
package learningpaths

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/learningpaths/internal/jobs"
	"tools.xdoubleu.com/apps/learningpaths/internal/repositories"
	"tools.xdoubleu.com/apps/learningpaths/internal/services"
	"tools.xdoubleu.com/apps/learningpaths/internal/todoist"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/crypto"
	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/jobqueue"
	"tools.xdoubleu.com/internal/platform/observability"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type LearningPaths struct {
	app.Base
	db        postgres.DB
	services  *services.Services
	jobQueue  *jobqueue.JobQueue
	resyncJob *jobs.TodoistResyncJob
}

// New builds the app. sealer encrypts per-user Todoist tokens. books and
// feeds must already be constructed (registration order in cmd/api/apps.go);
// only their exported methods are called, behind the services' lookup ports
// (docs/adr-0007-dashboard-app-owns-public-sharing.md).
func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sealer *crypto.Sealer,
	books BookSource,
	feeds FeedItemSource,
) *LearningPaths {
	//nolint:exhaustruct //services and jobs initialised below
	a := &LearningPaths{
		Base: app.NewBase(
			ctx,
			authService,
			logger,
			cfg,
		),
		db: db,
	}
	todoistConf := todoist.OAuthConfig(
		cfg.TodoistOAuthClientID, cfg.TodoistOAuthClientSecret, cfg.APIURL,
	)
	repos := repositories.New(db, sealer)
	a.services = services.New(
		a.Logger,
		repos.LearningPaths,
		repos.OAuthConnections,
		authService,
		todoistConf,
		bookPort{src: books},
		feedPort{src: feeds},
	)

	const amountOfWorkers = 1
	const jobQueueSize = 2
	a.jobQueue = jobqueue.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize, db)
	a.resyncJob = jobs.NewTodoistResyncJob(a.services.Todoist)

	return a
}

func (a *LearningPaths) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *LearningPaths) Start() error {
	return a.jobQueue.AddJob(
		observability.NewTrackedJob(a.resyncJob, a.db),
		func(_ string, _ bool, _ *time.Time) {},
	)
}

// Shutdown drains background tasks, then the job queue.
func (a *LearningPaths) Shutdown(ctx context.Context) error {
	return errors.Join(a.Base.Shutdown(ctx), a.jobQueue.Wait(ctx))
}

func (a *LearningPaths) GetName() string {
	return "learningpaths"
}

func (a *LearningPaths) GetDisplayName() string {
	return "Learning Paths"
}

// TodoistServiceForTest exposes the Todoist service to external tests.
func (a *LearningPaths) TodoistServiceForTest() *services.TodoistService {
	return a.services.Todoist
}
