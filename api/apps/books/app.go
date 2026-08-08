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
	// Services and Repositories are exported so integration tests can seed
	// data through the real service layer.
	Services       *services.Services
	Repositories   *repositories.Repositories
	profileShares  *sharedrepos.ProfileSharesRepository
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

	// Hardcover requires a token to work at all, so leave the client nil when
	// unset — the resync orchestration nil-checks every optional provider.
	var hardcoverClient hardcover.Client
	if cfg.HardcoverAPIKey == "" {
		logger.Warn(
			"HARDCOVER_API_KEY is not set — Hardcover metadata source is disabled",
		)
	} else {
		hardcoverClient = hardcover.New(logger, cfg.HardcoverAPIKey)
	}

	endpoint := "https://" + cfg.R2AccountID + ".r2.cloudflarestorage.com"
	webFetchClient := webfetch.New(logger)

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
	a.profileShares = sharedrepos.NewProfileSharesRepository(a.db)
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

// renameLegacyBooksSchema adopts a pre-2024 database: the app (and its
// schema) used to be called "books". goose's version table lives inside the
// schema, so renaming the schema carries the full migration history along —
// this must run before ApplyMigrationsFromFS creates an empty "reading"
// schema, or goose would try to re-run every migration from scratch. An
// empty "reading" schema left behind by a partial deploy is dropped first;
// a populated one means the rename already happened.
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

// renameLegacyReadingSchema is the inverse of renameLegacyBooksSchema, for
// the app's second rename (reading back to books, issue #736): a database
// still on the "reading" schema gets it renamed to "books", carrying
// goose's version table (and thus migration history) along. An empty
// "books" schema left behind by a partial deploy is dropped first; a
// populated one means this rename already happened. Kept alongside the
// original shim rather than replacing it, since a database that never ran
// the books→reading rename still needs that one first.
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

// RunStorageScanNow runs the R2 bucket scan synchronously, wrapped in the
// same TrackedJob used for the scheduled run so a manual trigger still shows
// up in global.job_runs / the Jobs card.
func (a *Books) RunStorageScanNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.storageScanJob, a.db).Run(ctx, a.Logger)
}

func (a *Books) GetName() string {
	return "books"
}

func (a *Books) GetDisplayName() string {
	return "Books"
}
