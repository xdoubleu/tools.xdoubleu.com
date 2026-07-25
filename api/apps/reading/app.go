package reading

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v4/pkg/threading"

	"tools.xdoubleu.com/apps/reading/internal/jobs"
	"tools.xdoubleu.com/apps/reading/internal/repositories"
	"tools.xdoubleu.com/apps/reading/internal/services"
	"tools.xdoubleu.com/apps/reading/pkg/arxiv"
	"tools.xdoubleu.com/apps/reading/pkg/hardcover"
	"tools.xdoubleu.com/apps/reading/pkg/objectstore"
	"tools.xdoubleu.com/apps/reading/pkg/unicat"
	"tools.xdoubleu.com/apps/reading/pkg/webfetch"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/observability"
	sharedrepos "tools.xdoubleu.com/internal/repositories"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Reading struct {
	app.Base
	db      postgres.DB
	clients Clients
	// Services and Repositories are exported so integration tests can seed
	// data through the real service layer.
	Services       *services.Services
	Repositories   *repositories.Repositories
	profileShares  *sharedrepos.ProfileSharesRepository
	jobQueue       *threading.JobQueue
	resyncBooksJob *jobs.ResyncMetadataJob
	storageScanJob *jobs.StorageScanJob
	feedPollJob    *jobs.FeedPollJob
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Reading {
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

	clients := Clients{
		UniCat:    unicat.New(logger),
		Hardcover: hardcoverClient,
		ObjectStore: objectstore.NewR2(
			endpoint,
			cfg.R2AccessKeyID,
			cfg.R2SecretKey,
			cfg.R2Bucket,
		),
		WebFetch:         webfetch.New(logger),
		Arxiv:            arxiv.New(logger),
		HTMLConvert:      nil, // default Calibre converter
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
) *Reading {
	//nolint:exhaustruct //jobQueue, Repositories, Services initialised below
	a := &Reading{
		Base:    app.NewBase(context.Background(), authService, logger, cfg),
		db:      db,
		clients: clients,
	}

	const amountOfWorkers = 2
	const jobQueueSize = 100
	a.jobQueue = threading.NewJobQueue(a.Ctx, logger, amountOfWorkers, jobQueueSize)

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
		clients.Arxiv,
		clients.HTMLConvert,
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
	a.feedPollJob = jobs.NewFeedPollJob(
		a.Services.Feeds,
		a.Services.WebSocket,
	)

	return a
}

func (a *Reading) Start() error {
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

	if err := a.jobQueue.AddJob(
		observability.NewTrackedJob(a.feedPollJob, a.db),
		a.Services.WebSocket.UpdateState,
	); err != nil {
		return err
	}

	a.Services.WebSocket.RegisterTopics(a.jobQueue.FetchJobIDs())
	return nil
}

func (a *Reading) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	if err := renameLegacyBooksSchema(ctx, db); err != nil {
		return err
	}
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

// renameLegacyBooksSchema adopts a pre-rename database: the app (and its
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

// RunStorageScanNow runs the R2 bucket scan synchronously, wrapped in the
// same TrackedJob used for the scheduled run so a manual trigger still shows
// up in global.job_runs / the Jobs card.
func (a *Reading) RunStorageScanNow(ctx context.Context) error {
	return observability.NewTrackedJob(a.storageScanJob, a.db).Run(ctx, a.Logger)
}

func (a *Reading) GetName() string {
	return "reading"
}

func (a *Reading) GetDisplayName() string {
	return "Reading"
}
