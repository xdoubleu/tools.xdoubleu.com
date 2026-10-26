package books

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/books/internal/jobs"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/observability"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/webfetch"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// userAgent identifies the books app's web fetches to origin servers.
const userAgent = "tools.xdoubleu.com reading-library bot"

type Books struct {
	app.Base
	db      postgres.DB
	clients Clients
	// Exported so integration tests can seed through the real service layer.
	Services       *services.Services
	Repositories   *repositories.Repositories
	jobQueue       *jobqueue.JobQueue
	resyncBooksJob *jobs.ResyncMetadataJob
	storageScanJob *jobs.StorageScanJob
	figuresJob     *jobs.FigureAnalysisJob
	kepubJob       *jobs.KEPUBConversionJob
}

func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Books {
	if cfg.R2AccountID == "" || cfg.R2AccessKeyID == "" ||
		cfg.R2SecretKey == "" || cfg.R2Bucket == "" {
		logger.Warn(
			"R2 object store is not fully configured — book file uploads will fail;" +
				" set R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET",
		)
	}

	// Hardcover needs a token; nil disables it (providers are nil-checked).
	var hardcoverClient hardcover.Client
	if cfg.HardcoverAPIKey == "" {
		logger.Warn(
			"HARDCOVER_API_KEY is not set — Hardcover metadata source is disabled",
		)
	} else {
		hardcoverClient = hardcover.New(logger, cfg.HardcoverAPIKey)
	}

	endpoint := "https://" + cfg.R2AccountID + ".r2.cloudflarestorage.com"
	webFetchClient := webfetch.New(logger, userAgent, cfg.Env != config.ProdEnv)

	clients := Clients{
		UniCat:    unicat.New(logger),
		Hardcover: hardcoverClient,
		ObjectStore: objectstore.NewR2(
			endpoint,
			cfg.R2AccessKeyID,
			cfg.R2SecretKey,
			cfg.R2Bucket,
		),
		WebFetch:         webFetchClient,
		KoboStoreBaseURL: "https://storeapi.kobo.com",
		PublicAPIBaseURL: cfg.APIURL,
	}

	return NewInner(ctx, authService, logger, cfg, db, clients)
}

func NewInner(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	clients Clients,
) *Books {
	//nolint:exhaustruct //jobQueue, Repositories, Services initialised below
	a := &Books{
		Base:    app.NewBase(ctx, authService, logger, cfg),
		db:      db,
		clients: clients,
	}

	const amountOfWorkers = 2
	const jobQueueSize = 100
	a.jobQueue = jobqueue.NewJobQueue(
		a.Ctx,
		logger,
		amountOfWorkers,
		jobQueueSize,
		a.db,
	)

	a.Repositories = repositories.New(a.db)
	a.Services = services.New(
		a.Ctx,
		logger,
		a.Background,
		a.Config,
		a.jobQueue,
		a.Repositories,
		clients.UniCat,
		clients.Hardcover,
		clients.ObjectStore,
		clients.WebFetch,
		authService,
	)
	a.resyncBooksJob = jobs.NewResyncMetadataJob(
		a.Services.Books,
		a.Services.WebSocket,
	)
	a.storageScanJob = jobs.NewStorageScanJob(
		clients.ObjectStore,
		a.Repositories.BookFiles,
		sharedrepos.NewStorageSnapshotsRepository(db),
	)
	a.figuresJob = jobs.NewFigureAnalysisJob(a.Services.Figures)
	a.kepubJob = jobs.NewKEPUBConversionJob(a.Services.KEPUBQueue)

	return a
}

func (a *Books) Start() error {
	if err := a.jobQueue.AddJob(
		observability.NewTrackedJob(a.resyncBooksJob, a.db),
		a.Services.WebSocket.UpdateState,
	); err != nil {
		return err
	}

	noop := func(_ string, _ bool, _ *time.Time) {}
	if err := a.jobQueue.AddJob(
		observability.NewTrackedJob(a.storageScanJob, a.db),
		noop,
	); err != nil {
		return err
	}

	a.Services.WebSocket.RegisterTopics(a.jobQueue.FetchJobIDs())
	return nil
}

// StartWorkers registers the memory-heavy jobs the workers role runs instead
// of Start's.
func (a *Books) StartWorkers() error {
	noop := func(_ string, _ bool, _ *time.Time) {}
	if err := a.jobQueue.AddJob(
		observability.NewTrackedJob(a.figuresJob, a.db),
		noop,
	); err != nil {
		return err
	}
	return a.jobQueue.AddJob(
		observability.NewTrackedJob(a.kepubJob, a.db),
		noop,
	)
}

// Shutdown drains background tasks, then the job queue.
func (a *Books) Shutdown(ctx context.Context) error {
	return errors.Join(a.Base.Shutdown(ctx), a.jobQueue.Wait(ctx))
}

func (a *Books) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

// StartStorageScan runs the scheduled R2 scan now on the job queue, so the
// run shows in job_runs and never overlaps a scheduled one.
func (a *Books) StartStorageScan() {
	a.jobQueue.ForceRun(a.storageScanJob.ID())
}

func (a *Books) GetName() string {
	return "books"
}

func (a *Books) GetDisplayName() string {
	return "Books"
}
