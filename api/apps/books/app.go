package books

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/books/internal/jobs"
	"tools.xdoubleu.com/apps/books/internal/repositories"
	"tools.xdoubleu.com/apps/books/internal/services"
	"tools.xdoubleu.com/apps/books/pkg/googlebooks"
	"tools.xdoubleu.com/apps/books/pkg/objectstore"
	"tools.xdoubleu.com/apps/books/pkg/openlibrary"
	"tools.xdoubleu.com/apps/books/pkg/unicat"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/observability"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Books struct {
	app.Base
	db      postgres.DB
	clients Clients
	// Services and Repositories are exported so integration tests can seed
	// data through the real service layer.
	Services       *services.Services
	Repositories   *repositories.Repositories
	jobQueue       *threading.JobQueue
	resyncBooksJob *jobs.ResyncOpenLibraryJob
	storageScanJob *jobs.StorageScanJob
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

	if cfg.GoogleBooksAPIKey == "" {
		logger.Warn(
			"GOOGLE_BOOKS_API_KEY is not set — Google Books will use the " +
				"unauthenticated tier (lower rate limit)",
		)
	}

	endpoint := "https://" + cfg.R2AccountID + ".r2.cloudflarestorage.com"

	clients := Clients{
		OpenLibrary: openlibrary.New(logger),
		GoogleBooks: googlebooks.New(logger, cfg.GoogleBooksAPIKey),
		UniCat:      unicat.New(logger),
		ObjectStore: objectstore.NewR2(
			endpoint,
			cfg.R2AccessKeyID,
			cfg.R2SecretKey,
			cfg.R2Bucket,
		),
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
	a.jobQueue = threading.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize)

	a.Repositories = repositories.New(a.db)
	a.Services = services.New(
		a.Ctx,
		logger,
		a.Config,
		a.jobQueue,
		a.Repositories,
		clients.OpenLibrary,
		clients.GoogleBooks,
		clients.UniCat,
		clients.ObjectStore,
		authService,
	)
	a.resyncBooksJob = jobs.NewResyncOpenLibraryJob(
		a.Services.Books,
		a.Services.WebSocket,
	)
	a.storageScanJob = jobs.NewStorageScanJob(
		clients.ObjectStore,
		a.Repositories.BookFiles,
		sharedrepos.NewStorageSnapshotsRepository(db),
	)

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

func (a *Books) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Books) GetName() string {
	return "books"
}

func (a *Books) GetDisplayName() string {
	return "Books"
}
