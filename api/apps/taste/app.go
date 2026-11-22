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
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste/internal/jobs"
	"tools.xdoubleu.com/apps/taste/internal/repositories"
	"tools.xdoubleu.com/apps/taste/internal/services"
	"tools.xdoubleu.com/apps/taste/internal/showsmigrations"
	"tools.xdoubleu.com/apps/taste/pkg/itunes"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/kit/openrouter"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/family"
	"tools.xdoubleu.com/internal/platform/jobqueue"
	"tools.xdoubleu.com/internal/platform/mcptools"
	"tools.xdoubleu.com/internal/platform/notifications"
	"tools.xdoubleu.com/internal/platform/observability"
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
	services *services.Services
	jobQueue *jobqueue.JobQueue
	genJob   jobs.GenerateProfilesJob
}

// New builds the app over sources, which must already be constructed
// (registration order in cmd/api/apps.go). Its favourite shows are looked up
// on iTunes and feed the podcasts domain.
func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	familyRepo *family.Repository,
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
	var a *Taste
	catalogs.Podcasts = func(
		ctx context.Context, userID, title string,
	) (*consumption.CatalogMatch, error) {
		return a.FindShow(ctx, userID, title)
	}
	a = NewInner(
		ctx, authService, logger, cfg, db, familyRepo, llm, sender, catalogs,
		sources...,
	)
	return a
}

// WithITunes swaps the iTunes client behind the favourite shows, for tests.
func (a *Taste) WithITunes(c itunes.Client) *Taste {
	a.services.Shows = services.NewShowService(repositories.NewShowsRepository(a.db), c)
	return a
}

// NewInner allows tests to inject a fake OpenRouter client and mail sender
// (nil sends no email); it counts as configured when cfg has a key.
func NewInner(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	familyRepo *family.Repository,
	llm openrouter.Client,
	mail EmailSender,
	catalogs Catalogs,
	sources ...Source,
) *Taste {
	if cfg.OpenRouterAPIKey == "" {
		logger.Warn("OPENROUTER_API_KEY not set; taste profiles are disabled")
	}
	//nolint:exhaustruct // services, jobQueue, genJob initialised below
	a := &Taste{
		Base: app.NewBase(ctx, authService, logger, cfg),
		db:   db,
		auth: authService,
	}
	//nolint:exhaustruct // the rest is built below
	svc := &services.Services{
		Shows: services.NewShowService(
			repositories.NewShowsRepository(db), itunes.New(),
		),
	}
	a.services = svc
	log := services.NewLogService(slices.Concat(sources, []Source{showSource{app: a}}))
	profiles := repositories.NewProfilesRepository(db)
	recs := repositories.NewRecommendationsRepository(db)
	svc.Profiles = services.NewProfileService(
		log, profiles, llm, cfg.TasteGeneratorModel, cfg.OpenRouterAPIKey != "",
	)
	svc.Picks = services.NewRecommendService(
		log, profiles, recs, catalogs, llm, cfg.TasteGeneratorModel, logger,
	)
	svc.Notify = services.NewNotifyService(
		repositories.NewPreferencesRepository(db), svc.Picks, mail, cfg.WebURL, logger,
	)
	svc.Reads = services.NewReadService(log, svc.Picks)
	svc.Scope = services.NewScopeService(familyRepo, mcpAccess{})

	const amountOfWorkers = 1
	const jobQueueSize = 10
	a.jobQueue = jobqueue.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize, db)
	a.genJob = jobs.NewGenerateProfilesJob(
		authService, svc.Profiles, svc.Picks, svc.Notify, a.GetName(),
	).WithShared(sharedSubjects(sources))
	return a
}

// mcpAccess reads the caller's app access from the request context.
type mcpAccess struct{}

func (mcpAccess) CanAccess(ctx context.Context, appName string) bool {
	return mcptools.RequireAppAccess(ctx, appName) == nil
}

// Shutdown drains background tasks, then the job queue.
func (a *Taste) Shutdown(ctx context.Context) error {
	return errors.Join(a.Base.Shutdown(ctx), a.jobQueue.Wait(ctx))
}

// ApplyMigrations also runs the favourite shows' migrations, which keep their
// own "podcasts" schema, ahead of taste's.
func (a *Taste) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	if err := a.ApplyMigrationsFromFS(
		ctx, db, showsmigrations.FS, "podcasts",
	); err != nil {
		return err
	}
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
