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
	"tools.xdoubleu.com/apps/factory/internal/slack"
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
	github        services.GitHub
	notifier      services.Notifier
	slackURL      string
	webURL        string
}

// WithJobAccounts makes every claimed job run as an agent account of its own,
// confined to its task. Without it a job gets no MCP identity.
func WithJobAccounts(accounts services.JobAccounts) Option {
	return func(a *Factory) { a.jobAccounts = accounts }
}

// WithGitHub lets a running job fetch a short-lived token to push its branch and
// open its pull request. Without it jobs end in Needs you with the work on the Mac.
func WithGitHub(github services.GitHub) Option {
	return func(a *Factory) { a.github = github }
}

// WithNotifier posts job outcomes and runner pairings to a notifier (Slack), with
// task links under webURL. Without it nothing is posted.
func WithNotifier(notifier services.Notifier, webURL string) Option {
	return func(a *Factory) { a.notifier, a.webURL = notifier, webURL }
}

// WithSlack posts job outcomes and runner pairings to the Slack incoming webhook,
// with task links under webURL. An empty address posts nothing; one that isn't
// https is refused with a warning.
func WithSlack(webhookURL, webURL string) Option {
	return func(a *Factory) { a.slackURL, a.webURL = webhookURL, webURL }
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
		notifier:      nil,
		slackURL:      "",
		webURL:        "",
	}
	for _, opt := range opts {
		opt(a)
	}
	a.connectSlack(logger)
	a.services = services.New(
		repositories.New(db), authService, a.jobAccounts, a.github, a.notifier, a.webURL,
	)
	return a
}

// connectSlack turns a webhook address into the notifier, unless a test set one.
func (a *Factory) connectSlack(logger *slog.Logger) {
	if a.notifier != nil || a.slackURL == "" {
		return
	}
	sender := slack.New(a.slackURL, logger)
	if sender == nil {
		logger.Warn("FACTORY_SLACK_WEBHOOK_URL is not an https address; not posting")
		return
	}
	a.notifier = sender
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
	return errors.Join(
		a.jobQueue.AddJob(a.pruneJob(), noop),
		a.jobQueue.AddJob(a.pollJob(), noop),
		a.jobQueue.AddJob(a.checkRunnerJob(), noop),
	)
}

func (a *Factory) pollJob() threading.Job {
	poll := jobs.NewPollPullRequestsJob(a.services.Jobs)
	return observability.NewTrackedJob(poll, a.db)
}

func (a *Factory) checkRunnerJob() threading.Job {
	check := jobs.NewCheckRunnerJob(a.services.Jobs)
	return observability.NewTrackedJob(check, a.db)
}

// CheckRunnerNow looks for a job waiting on an offline runner once, outside the
// schedule.
func (a *Factory) CheckRunnerNow(ctx context.Context) error {
	return a.checkRunnerJob().Run(ctx, a.Logger)
}

// PollPullRequestsNow reads every open pull request once, outside the schedule.
func (a *Factory) PollPullRequestsNow(ctx context.Context) error {
	return a.pollJob().Run(ctx, a.Logger)
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
