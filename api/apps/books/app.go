package books

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/books/internal/jobs"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/hardcover"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/apps/books/pkg/webfetch"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/jobqueue"
	"tools.xdoubleu.com/internal/observability"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

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
	webFetchClient := webfetch.New(logger, cfg.Env != config.ProdEnv)

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

	return NewInner(authService, logger, cfg, db, clients)
}

func NewInner(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	clients Clients,
) *Books {
	//nolint:exhaustruct //jobQueue, Repositories, Services initialised below
	a := &Books{
		Base:    app.NewBase(context.Background(), authService, logger, cfg),
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

func (a *Books) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

// RunStorageScanNow runs the R2 scan synchronously under the scheduled job's
// TrackedJob, so manual runs show in job_runs.
func (a *Books) RunStorageScanNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.storageScanJob, a.db).Run(ctx, a.Logger)
}

func (a *Books) GetName() string {
	return "books"
}

func (a *Books) GetDisplayName() string {
	return "Books"
}
