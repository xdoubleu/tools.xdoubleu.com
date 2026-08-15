package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	gotrue "github.com/supabase-community/auth-go"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/contacts"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/jobqueue"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/observability"
	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
	"tools.xdoubleu.com/sentrytools"
)

//go:embed migrations/*.sql
var globalMigrations embed.FS

type Application struct {
	ctx                           context.Context
	logger                        *slog.Logger
	config                        config.Config
	db                            *pgxpool.Pool
	auth                          *auth.GoTrueService
	contacts                      contacts.Service
	apps                          *Apps
	booksApp                      storageScanRunner
	appUsersRepo                  *repositories.AppUsersRepository
	profileSharesRepo             *repositories.ProfileSharesRepository
	usage                         *observability.UsageRecorder
	jobRunsRepo                   *repositories.JobRunsRepository
	usageRepo                     *repositories.UsageRepository
	storageRepo                   *repositories.StorageSnapshotsRepository
	dbStatsRepo                   *repositories.DBStatsRepository
	githubClient                  github.Client
	sentryClient                  sentryapi.Client
	doClient                      digitalocean.Client
	oauthConnRepo                 *repositories.OAuthConnectionsRepository
	oauthState                    *oauthconn.StateStore
	issueNotifierJob              *jobs.IssueNotifierJob
	transactionLatencyRepo        *repositories.TransactionLatencyRepository
	transactionLatencySnapshotJob *jobs.TransactionLatencySnapshotJob
	globalJobQueue                *jobqueue.JobQueue
}

//	@title			tools
//	@version		1.0
//	@license.name	GPL-3.0
//	@Accept			json
//	@Produce		json

const (
	dbMaxConns           = 25
	dbMaxIdleTime        = "15m"
	dbConnectTimeoutSecs = 10
	dbRetrySleep         = 2 * time.Second
	dbMaxRetryDuration   = 20 * time.Second
	httpReadTimeout      = 5 * time.Second
	httpWriteTimeout     = 10 * time.Second
	// migrationLockKey identifies the advisory lock that serializes
	// migration runs across concurrently starting replicas.
	migrationLockKey = 20260101
	// usageFlushInterval is how often accumulated request counts are
	// written to global.usage_daily.
	usageFlushInterval = time.Minute
	// globalJobQueueWorkers/globalJobQueueSize size the job queue backing
	// cross-app jobs (the issue notifier, issue #561, and the daily
	// transaction-latency snapshot, issue #848).
	globalJobQueueWorkers = 1
	globalJobQueueSize    = 10
)

// migrationLockTimeout bounds how long a starting replica waits for the
// migration advisory lock before failing loudly, so a lock left held by a
// stale connection from a prior replica can't hang startup silently forever.
// A var (not const) so tests can shrink it instead of waiting out the real
// timeout.
//
//nolint:gochecknoglobals //test seam, see comment above
var migrationLockTimeout = 20 * time.Second

// newDBPool opens the shared pgx pool with the app's real connect
// parameters; factored out so tests can exercise the same argument list
// TestMain uses to spin up its own test-DB pool.
func newDBPool(logger *slog.Logger, dsn string) (*pgxpool.Pool, error) {
	return postgres.Connect(
		logger, dsn, dbMaxConns, dbMaxIdleTime,
		dbConnectTimeoutSecs, dbRetrySleep, dbMaxRetryDuration,
	)
}

func main() {
	cfg := config.New(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	logger := slog.New(sentrytools.NewLogHandler(cfg.Env,
		slog.NewTextHandler(os.Stdout, nil)))
	// Code that can't receive the injected logger falls back to
	// slog.Default(); route it through the Sentry handler too.
	slog.SetDefault(logger)
	db, err := newDBPool(logger, cfg.DBDsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	supabase := gotrue.New(
		cfg.SupabaseProjRef,
		cfg.SupabaseAPIKey,
	)

	app := NewApplication(logger, cfg, db, supabase)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      app.Routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
	}
	err = httptools.Serve(logger, srv, cfg.Env)
	if err != nil {
		logger.Error("failed to serve server", essentialogger.ErrAttr(err))
	}
}

// newOAuthSealer builds the AES-GCM sealer used to encrypt stored OAuth
// tokens (issue #440). Returns nil if ENCRYPTION_KEY isn't set — the
// observability integrations then simply can't be connected until it is; the
// rest of the app still starts.
func newOAuthSealer(logger *slog.Logger, config config.Config) *crypto.Sealer {
	if config.EncryptionKey == "" {
		logger.Warn(
			"ENCRYPTION_KEY not set — GitHub/Sentry/DigitalOcean " +
				"OAuth connections cannot be stored",
		)
		return nil
	}
	sealer, err := crypto.New(config.EncryptionKey)
	if err != nil {
		panic(err)
	}
	return sealer
}

// newContactsService wires the contacts service to a notifications.Service
// backed by the Resend mailer (issue #383) so a contact request emails its
// recipient without blocking the request on the Resend round trip (issue
// #923). notificationsSvc is also returned for reuse by NewApps (feeds) and
// IssueNotifierJob, so every mail notification in the app shares the one
// FIFO delivery queue.
func newContactsService(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	repo *repositories.ContactsRepository,
	authSvc auth.Service,
) (contacts.Service, *notifications.Service) {
	mailClient := mailer.New(
		config.ResendAPIKey,
		config.EmailFrom,
		config.NotifyEmailTo,
	)
	notificationsSvc := notifications.New(ctx, logger, mailClient)
	return contacts.New(repo, authSvc, notificationsSvc, config.WebURL, logger),
		notificationsSvc
}

// newObservabilityClients builds the three external observability clients,
// each resolving its bearer token from oauthConnRepo via oauthconn.TokenFunc
// instead of a static config value (issue #440).
func newObservabilityClients(
	logger *slog.Logger,
	config config.Config,
	oauthConnRepo *repositories.OAuthConnectionsRepository,
) (github.Client, sentryapi.Client, digitalocean.Client) {
	if config.GithubOAuthClientID == "" || config.GithubOAuthClientSecret == "" {
		logger.Warn(
			"GITHUB_OAUTH_CLIENT_ID/SECRET not set — GitHub OAuth connect will fail",
		)
	}
	githubClient := github.New(
		logger,
		oauthconn.NewTokenFunc(
			oauthConnRepo, models.OAuthProviderGithub,
			github.OAuthConfig(
				config.GithubOAuthClientID, config.GithubOAuthClientSecret,
				config.APIURL,
			),
		),
		oauthConnRepo,
	)
	if config.SentryOAuthClientID == "" || config.SentryOAuthClientSecret == "" {
		logger.Warn(
			"SENTRY_OAUTH_CLIENT_ID/SECRET not set — Sentry OAuth connect will fail",
		)
	}
	sentryClient := sentryapi.New(
		logger,
		oauthconn.NewTokenFunc(
			oauthConnRepo, models.OAuthProviderSentry,
			sentryapi.OAuthConfig(
				config.SentryOAuthClientID, config.SentryOAuthClientSecret,
				config.APIURL,
			),
		),
		oauthConnRepo,
	)
	if config.DOOAuthClientID == "" || config.DOOAuthClientSecret == "" {
		logger.Warn(
			"DO_OAUTH_CLIENT_ID/SECRET not set — DigitalOcean OAuth connect will fail",
		)
	}
	doClient := digitalocean.New(
		logger,
		oauthconn.NewTokenFunc(
			oauthConnRepo, models.OAuthProviderDigitalOcean,
			digitalocean.OAuthConfig(
				config.DOOAuthClientID, config.DOOAuthClientSecret,
				config.APIURL,
			),
		),
		oauthConnRepo,
	)
	return githubClient, sentryClient, doClient
}

// newCrossAppJobs builds the jobs registered directly on
// Application.globalJobQueue by startCrossAppJobs — cross-app observability
// concerns, not scoped to one apps/<name>. transactionLatencyRepo is
// returned alongside its job since the Connect/MCP handlers also read from
// it directly.
func newCrossAppJobs(
	db *pgxpool.Pool,
	sentryClient sentryapi.Client,
	doClient digitalocean.Client,
	githubClient github.Client,
	notificationsSvc *notifications.Service,
) (
	*jobs.IssueNotifierJob,
	*repositories.TransactionLatencyRepository,
	*jobs.TransactionLatencySnapshotJob,
) {
	notifiedIssuesRepo := repositories.NewNotifiedIssuesRepository(db)
	issueNotifierJob := jobs.NewIssueNotifierJob(
		sentryClient, doClient, githubClient, notificationsSvc, notifiedIssuesRepo,
	)

	transactionLatencyRepo := repositories.NewTransactionLatencyRepository(db)
	transactionLatencySnapshotJob := jobs.NewTransactionLatencySnapshotJob(
		sentryClient, transactionLatencyRepo,
	)

	return issueNotifierJob, transactionLatencyRepo, transactionLatencySnapshotJob
}

// startCrossAppJobs registers every job living directly on app.globalJobQueue
// — cross-app observability concerns, not scoped to one apps/<name> — rather
// than one apps/<name>/app.go's own Start().
func startCrossAppJobs(app *Application) error {
	noopCallback := func(_ string, _ bool, _ *time.Time) {}
	if err := app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.issueNotifierJob, app.db), noopCallback,
	); err != nil {
		return err
	}
	return app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.transactionLatencySnapshotJob, app.db),
		noopCallback,
	)
}

func NewApplication(
	logger *slog.Logger,
	config config.Config,
	db *pgxpool.Pool,
	supabaseClient gotrue.Client,
) *Application {
	ctx := context.Background()

	sentryHub, err := sentrytools.Init(config.Env, sentry.ClientOptions{
		Dsn:              config.SentryDsn,
		Environment:      config.Env,
		Release:          config.Release,
		EnableTracing:    true,
		TracesSampleRate: config.SampleRate,
		SampleRate:       config.SampleRate,
	})
	if err != nil {
		panic(err)
	}
	if sentryHub != nil {
		ctx = sentry.SetHubOnContext(ctx, sentryHub)
	}

	appUsersRepo := repositories.NewAppUsersRepository(db)
	contactsRepo := repositories.NewContactsRepository(db)
	authSvc := auth.NewService(config, supabaseClient, appUsersRepo)
	authSvc.SignInRenderer = func(
		w http.ResponseWriter, _ *http.Request, _ string,
	) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}

	contactsSvc, notificationsSvc := newContactsService(
		ctx, logger, config, contactsRepo, authSvc,
	)

	oauthConnRepo := repositories.NewOAuthConnectionsRepository(
		db, newOAuthSealer(logger, config),
	)
	githubClient, sentryClient, doClient := newObservabilityClients(
		logger, config, oauthConnRepo,
	)

	issueNotifierJob, transactionLatencyRepo, transactionLatencySnapshotJob :=
		newCrossAppJobs(db, sentryClient, doClient, githubClient, notificationsSvc)

	//nolint:exhaustruct //apps/booksApp are set after construction, see below
	app := &Application{
		ctx:                           ctx,
		logger:                        logger,
		config:                        config,
		db:                            db,
		auth:                          authSvc,
		contacts:                      contactsSvc,
		appUsersRepo:                  appUsersRepo,
		profileSharesRepo:             repositories.NewProfileSharesRepository(db),
		usage:                         observability.NewUsageRecorder(logger, db),
		jobRunsRepo:                   repositories.NewJobRunsRepository(db),
		usageRepo:                     repositories.NewUsageRepository(db),
		storageRepo:                   repositories.NewStorageSnapshotsRepository(db),
		dbStatsRepo:                   repositories.NewDBStatsRepository(db),
		oauthConnRepo:                 oauthConnRepo,
		oauthState:                    oauthconn.NewStateStore(),
		githubClient:                  githubClient,
		sentryClient:                  sentryClient,
		doClient:                      doClient,
		issueNotifierJob:              issueNotifierJob,
		transactionLatencyRepo:        transactionLatencyRepo,
		transactionLatencySnapshotJob: transactionLatencySnapshotJob,
		globalJobQueue: jobqueue.NewJobQueue(
			ctx, logger, globalJobQueueWorkers, globalJobQueueSize, db,
		),
	}

	// One tracing wrapper for every app's queries; migrations keep the raw pool.
	spanDB := postgres.NewSpanDB(db)
	app.apps, app.booksApp = NewApps(
		app.auth, logger, config, spanDB, notificationsSvc, appUsersRepo,
	)

	err = app.ApplyMigrations(db)
	if err != nil {
		panic(err)
	}

	// Flush accumulated request counts to global.usage_daily periodically;
	// the loop lives for the process lifetime (ctx is context.Background).
	app.usage.Start(ctx, usageFlushInterval)

	if err = startCrossAppJobs(app); err != nil {
		panic(err)
	}

	for _, a := range *app.apps {
		err = a.Start()
		if err != nil {
			panic(err)
		}
	}

	return app
}

func (app *Application) ApplyMigrations(db *pgxpool.Pool) error {
	// Session-level advisory lock held on a dedicated connection, so two
	// replicas rolling out at the same time never run migrations concurrently.
	lockConn, err := db.Acquire(app.ctx)
	if err != nil {
		return err
	}
	defer lockConn.Release()

	lockCtx, cancel := context.WithTimeout(app.ctx, migrationLockTimeout)
	defer cancel()

	app.logger.Info("acquiring migration lock")
	if _, err = lockConn.Exec(
		lockCtx, "SELECT pg_advisory_lock($1)", migrationLockKey,
	); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	app.logger.Info("acquired migration lock")
	defer func() {
		_, _ = lockConn.Exec(
			app.ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey,
		)
	}()

	if err = app.applyGlobalMigrations(db); err != nil {
		return err
	}
	return app.apps.ApplyMigrations(app.ctx, db)
}

func (app *Application) applyGlobalMigrations(db *pgxpool.Pool) error {
	if _, err := db.Exec(app.ctx, "CREATE SCHEMA IF NOT EXISTS global"); err != nil {
		return err
	}

	goose.SetTableName("global.goose_db_version")
	goose.SetLogger(slog.NewLogLogger(app.logger.Handler(), slog.LevelInfo))
	goose.SetBaseFS(globalMigrations)

	if err := goose.SetDialect(string(goose.DialectPostgres)); err != nil {
		return err
	}

	migrationsDB := stdlib.OpenDBFromPool(db)
	return goose.Up(migrationsDB, "migrations")
}
