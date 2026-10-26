// Package taste builds a per-user consumption log across every media app, its
// cross-domain statistics, and an LLM-written taste profile. A family's shared
// movies & series list gets its own profile, treating the family as one
// person. It reaches other apps only through their exported methods:
// ListConsumption to read, Find* to look candidates up, and Want* to add an
// accepted pick to a backlog (the only write).
package taste

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/repositories"
	"tools.xdoubleu.com/apps/taste/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/observability"
	"tools.xdoubleu.com/internal/openrouter"
	sharedrepositories "tools.xdoubleu.com/internal/repositories"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Source is an app exporting its consumed items.
type Source = services.Source

// EmailSender queues the new-picks email.
type EmailSender = services.EmailSender

// Catalogs look recommendation candidates up in each domain's catalog.
type Catalogs = services.Catalogs

type Taste struct {
	app.Base
	db       postgres.DB
	auth     auth.Service
	family   *sharedrepositories.FamilyRepository
	log      *services.LogService
	profiles *services.ProfileService
	picks    *services.RecommendService
	notify   *services.NotifyService
	jobQueue *jobqueue.JobQueue
	genJob   jobs.GenerateProfilesJob
}

// New builds the app over sources, which must already be constructed
// (registration order in cmd/api/apps.go).
func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	mail *notifications.Service,
	catalogs Catalogs,
	sources ...Source,
) *Taste {
	llm := openrouter.New(cfg.OpenRouterAPIKey, cfg.WebURL, logger)
	// A nil *Service must not become a non-nil EmailSender.
	var sender EmailSender
	if mail != nil {
		sender = mail
	}
	return NewInner(ctx, authService, logger, cfg, db, llm, sender, catalogs, sources...)
}

// NewInner allows tests to inject a fake OpenRouter client and mail sender
// (nil sends no email); it counts as configured when cfg has a key.
func NewInner(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	llm openrouter.Client,
	mail EmailSender,
	catalogs Catalogs,
	sources ...Source,
) *Taste {
	if cfg.OpenRouterAPIKey == "" {
		logger.Warn("OPENROUTER_API_KEY not set; taste profiles are disabled")
	}
	//nolint:exhaustruct // jobQueue, genJob initialised below
	a := &Taste{
		Base:   app.NewBase(ctx, authService, logger, cfg),
		db:     db,
		auth:   authService,
		family: sharedrepositories.NewFamilyRepository(db),
		log:    services.NewLogService(sources),
	}
	profiles := repositories.NewProfilesRepository(db)
	a.profiles = services.NewProfileService(
		a.log, profiles, llm, cfg.TasteGeneratorModel, cfg.OpenRouterAPIKey != "",
	)
	a.picks = services.NewRecommendService(
		a.log, profiles, repositories.NewRecommendationsRepository(db), catalogs,
		llm, cfg.TasteGeneratorModel, logger,
	)
	a.notify = services.NewNotifyService(
		repositories.NewPreferencesRepository(db), a.picks, mail, cfg.WebURL, logger,
	)

	const amountOfWorkers = 1
	const jobQueueSize = 10
	a.jobQueue = jobqueue.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize, db)
	a.genJob = jobs.NewGenerateProfilesJob(
		authService, a.profiles, a.picks, a.notify, a.GetName(),
	).WithShared(sharedSubjects(sources))
	return a
}

// Shutdown drains background tasks, then the job queue.
func (a *Taste) Shutdown(ctx context.Context) error {
	return errors.Join(a.Base.Shutdown(ctx), a.jobQueue.Wait(ctx))
}

func (a *Taste) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Taste) Start() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	return a.jobQueue.AddJob(observability.NewTrackedJob(a.genJob, a.db), noop)
}

// GenerateProfilesNow runs the profile job once, outside its schedule.
func (a *Taste) GenerateProfilesNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.genJob, a.db).Run(ctx, a.Logger)
}

func (a *Taste) GetName() string {
	return "taste"
}

func (a *Taste) GetDisplayName() string {
	return "Taste"
}

// sharedSubjects lists the shared lists of every source that keeps one.
func sharedSubjects(sources []Source) jobs.SharedLister {
	type sharer interface {
		ListSharedSubjects(ctx context.Context) ([]string, error)
	}
	return func(ctx context.Context) ([]string, error) {
		var out []string
		for _, src := range sources {
			if s, ok := src.(sharer); ok {
				subjects, err := s.ListSharedSubjects(ctx)
				if err != nil {
					return nil, err
				}
				out = append(out, subjects...)
			}
		}
		return out, nil
	}
}
