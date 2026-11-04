// Package factory is the admin-only task board of the software factory.
package factory

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/factory/internal/jobs"
	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/apps/factory/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/observability"
	"tools.xdoubleu.com/internal/threading"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Factory struct {
	app.Base
	services *services.Services
	db       postgres.DB
	jobQueue *jobqueue.JobQueue
	// stepUp checks the TOTP code that creating a runner needs.
	stepUp        StepUp
	heartbeatHold time.Duration
	jobAccounts   services.JobAccounts
	github        services.JobGitHub
}

// WithJobAccounts makes every claimed job run as an agent account of its own,
// confined to its task. Without it a job gets no MCP identity.
func WithJobAccounts(accounts services.JobAccounts) Option {
	return func(a *Factory) { a.jobAccounts = accounts }
}

// WithGitHub lets a running job fetch a short-lived token to push its branch and
// open its pull request. Without it jobs end in Needs you with the work on the Mac.
func WithGitHub(github services.JobGitHub) Option {
	return func(a *Factory) { a.github = github }
}

// Option adjusts a Factory.
type Option func(*Factory)

// WithStepUp sets the check run before a runner is created. Without it,
// creating a runner is refused.
func WithStepUp(stepUp StepUp) Option {
	return func(a *Factory) { a.stepUp = stepUp }
}

// WithHeartbeatHold sets how long a Heartbeat is held open; tests shorten it.
func WithHeartbeatHold(d time.Duration) Option {
	return func(a *Factory) { a.heartbeatHold = d }
}

const (
	amountOfWorkers = 1
	jobQueueSize    = 10
)

func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	opts ...Option,
) *Factory {
	a := &Factory{
		Base:          app.NewBase(ctx, authService, logger, cfg),
		services:      nil,
		db:            db,
		jobQueue:      jobqueue.NewJobQueue(ctx, logger, amountOfWorkers, jobQueueSize, db),
		stepUp:        nil,
		heartbeatHold: DefaultHeartbeatHold,
		jobAccounts:   nil,
		github:        nil,
	}
	for _, opt := range opts {
		opt(a)
	}
	a.services = services.New(repositories.New(db), authService, a.jobAccounts, a.github)
	return a
}

func (a *Factory) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Factory) GetName() string {
	return "factory"
}

func (a *Factory) GetDisplayName() string {
	return "Factory"
}

func (a *Factory) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(a.pruneJob(), noop)
}

func (a *Factory) pruneJob() threading.Job {
	prune := jobs.NewPruneJobEventsJob(a.services.Jobs)
	return observability.NewTrackedJob(prune, a.db)
}

// PruneEventsNow runs the job-log retention job once, outside its schedule.
func (a *Factory) PruneEventsNow(ctx context.Context) error {
	return a.pruneJob().Run(ctx, a.Logger)
}

// Shutdown drains background tasks, then the job queue.
func (a *Factory) Shutdown(ctx context.Context) error {
	return errors.Join(a.Base.Shutdown(ctx), a.jobQueue.Wait(ctx))
}
