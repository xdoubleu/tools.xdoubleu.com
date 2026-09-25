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
	if err := renameLegacyBooksSchema(ctx, db); err != nil {
		return err
	}
	if err := renameLegacyReadingSchema(ctx, db); err != nil {
		return err
	}
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

// renameLegacyBooksSchema renames an old "books" schema to "reading", carrying
// goose's version table along. Must run before ApplyMigrationsFromFS creates an
// empty "reading" schema; an empty one from a partial deploy is dropped first.
func renameLegacyBooksSchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		DO $$
		BEGIN
		    IF EXISTS (
		        SELECT 1 FROM information_schema.schemata
		        WHERE schema_name = 'books'
		    ) THEN
		        IF EXISTS (
		            SELECT 1 FROM information_schema.schemata
		            WHERE schema_name = 'reading'
		        ) AND NOT EXISTS (
		            SELECT 1 FROM information_schema.tables
		            WHERE table_schema = 'reading'
		        ) THEN
		            DROP SCHEMA reading;
		        END IF;
		        IF NOT EXISTS (
		            SELECT 1 FROM information_schema.schemata
		            WHERE schema_name = 'reading'
		        ) THEN
		            ALTER SCHEMA books RENAME TO reading;
		        END IF;
		    END IF;
		END $$;
	`)
	return err
}

// renameLegacyReadingSchema renames "reading" back to "books" the same way.
// Both run, since a database may still need the first rename.
func renameLegacyReadingSchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		DO $$
		BEGIN
		    IF EXISTS (
		        SELECT 1 FROM information_schema.schemata
		        WHERE schema_name = 'reading'
		    ) THEN
		        IF EXISTS (
		            SELECT 1 FROM information_schema.schemata
		            WHERE schema_name = 'books'
		        ) AND NOT EXISTS (
		            SELECT 1 FROM information_schema.tables
		            WHERE table_schema = 'books'
		        ) THEN
		            DROP SCHEMA books;
		        END IF;
		        IF NOT EXISTS (
		            SELECT 1 FROM information_schema.schemata
		            WHERE schema_name = 'books'
		        ) THEN
		            ALTER SCHEMA reading RENAME TO books;
		        END IF;
		    END IF;
		END $$;
	`)
	return err
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
