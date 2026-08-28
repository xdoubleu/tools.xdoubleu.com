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

	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/contacts"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/jobqueue"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/oauth2as"
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
	auth                          *auth.LocalService
	authSealer                    *crypto.Sealer
	oauth2as                      *oauth2asWiring
	contacts                      contacts.Service
	apps                          *Apps
	booksApp                      storageScanRunner
	feedsApp                      unhealthyFeedLister
	appUsersRepo                  *repositories.AppUsersRepository
	profileSharesRepo             *repositories.ProfileSharesRepository
	usage                         *observability.UsageRecorder
	jobRunsRepo                   *repositories.JobRunsRepository
	usageRepo                     *repositories.UsageRepository
	storageRepo                   *repositories.StorageSnapshotsRepository
	dbStatsRepo                   *repositories.DBStatsRepository
	dbSizeSamplesRepo             *repositories.DBSizeSamplesRepository
	dbSizeSnapshotJob             *jobs.DBSizeSnapshotJob
	hostMetricsRepo               *repositories.HostMetricsRepository
	logsRepo                      *repositories.LogsRepository
	notificationSettingsRepo      *repositories.NotificationSettingsRepository
	githubClient                  github.Client
	sentryClient                  sentryapi.Client
	oauthConnRepo                 *repositories.OAuthConnectionsRepository
	oauthState                    *oauthconn.StateStore
	issueNotifierJob              *jobs.IssueNotifierJob
	transactionLatencyRepo        *repositories.TransactionLatencyRepository
	transactionLatencySnapshotJob *jobs.TransactionLatencySnapshotJob
	weeklyDigestJob               *jobs.WeeklyDigestJob
	hostMetricsSnapshotJob        *jobs.HostMetricsSnapshotJob
	workflowRunsRepo              *repositories.WorkflowRunsRepository
	workflowRunsSnapshotJob       *jobs.WorkflowRunsSnapshotJob
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
	// hostMetricsScrapeTimeout bounds one node_exporter HTTP scrape.
	hostMetricsScrapeTimeout = 5 * time.Second
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

	bootLogger := slog.New(sentrytools.NewLogHandler(cfg.Env,
		slog.NewTextHandler(os.Stdout, nil)))
	db, err := newDBPool(bootLogger, cfg.DBDsn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// The DB pool needs to exist before a handler can tee logs into
	// global.log_entries, so this handler chain is built after it, rather
	// than alongside bootLogger above.
	logger := slog.New(observability.NewLogRepoHandler(
		sentrytools.NewLogHandler(cfg.Env, slog.NewTextHandler(os.Stdout, nil)),
		repositories.NewLogsRepository(db),
	))
	// Code that can't receive the injected logger falls back to
	// slog.Default(); route it through the same handler chain too.
	slog.SetDefault(logger)

	app := NewApplication(logger, cfg, db)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      stripAPIPathPrefix(app.Routes()),
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
// tokens (issue #440) and TOTP secrets (issue #1039). Returns nil if
// ENCRYPTION_KEY isn't set — the observability integrations and TOTP
// enrollment then simply can't be used until it is; the rest of the app
// still starts.
func newOAuthSealer(logger *slog.Logger, config config.Config) *crypto.Sealer {
	if config.EncryptionKey == "" {
		logger.Warn(
			"ENCRYPTION_KEY not set — GitHub/Sentry OAuth connections and " +
				"TOTP enrollment cannot be stored",
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

// newObservabilityClients builds the two external observability clients,
// each resolving its bearer token from oauthConnRepo via oauthconn.TokenFunc
// instead of a static config value (issue #440).
func newObservabilityClients(
	logger *slog.Logger,
	config config.Config,
	oauthConnRepo *repositories.OAuthConnectionsRepository,
) (github.Client, sentryapi.Client) {
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
	return githubClient, sentryClient
}

// newCrossAppJobs builds the jobs registered directly on
// Application.globalJobQueue by startCrossAppJobs — cross-app observability
// concerns, not scoped to one apps/<name>. transactionLatencyRepo is
// returned alongside its job since the Connect/MCP handlers also read from
// it directly.
func newCrossAppJobs(
	db *pgxpool.Pool,
	sentryClient sentryapi.Client,
	githubClient github.Client,
	notificationsSvc *notifications.Service,
	notificationSettingsRepo *repositories.NotificationSettingsRepository,
	storageSnapshotsRepo *repositories.StorageSnapshotsRepository,
) (
	*jobs.IssueNotifierJob,
	*repositories.TransactionLatencyRepository,
	*jobs.TransactionLatencySnapshotJob,
) {
	notifiedIssuesRepo := repositories.NewNotifiedIssuesRepository(db)
	issueNotifierJob := jobs.NewIssueNotifierJob(
		sentryClient, githubClient, notificationsSvc, notifiedIssuesRepo,
		notificationSettingsRepo, storageSnapshotsRepo,
	)

	transactionLatencyRepo := repositories.NewTransactionLatencyRepository(db)
	transactionLatencySnapshotJob := jobs.NewTransactionLatencySnapshotJob(
		sentryClient, transactionLatencyRepo,
	)

	return issueNotifierJob, transactionLatencyRepo, transactionLatencySnapshotJob
}

// newWorkflowRunsSnapshotJob builds the workflow-run history job (issue
// #1217), reusing the same notifiedIssuesRepo dedup table IssueNotifierJob
// uses for its own main-branch-failure alert.
func newWorkflowRunsSnapshotJob(
	db *pgxpool.Pool,
	githubClient github.Client,
	notificationsSvc *notifications.Service,
	notificationSettingsRepo *repositories.NotificationSettingsRepository,
) (*repositories.WorkflowRunsRepository, *jobs.WorkflowRunsSnapshotJob) {
	workflowRunsRepo := repositories.NewWorkflowRunsRepository(db)
	notifiedIssuesRepo := repositories.NewNotifiedIssuesRepository(db)
	workflowRunsSnapshotJob := jobs.NewWorkflowRunsSnapshotJob(
		githubClient,
		workflowRunsRepo,
		notificationsSvc,
		notifiedIssuesRepo,
		notificationSettingsRepo,
	)
	return workflowRunsRepo, workflowRunsSnapshotJob
}

// feedsHealthAdapter adapts *feeds.Feeds to jobs.unhealthyFeedLister so
// WeeklyDigestJob (internal/observability/jobs) never imports apps/feeds
// directly — feeds.UnhealthyFeed and jobs.UnhealthyFeed are structurally
// identical but distinct types, so main.go (the composition root) is what
// bridges them.
type feedsHealthAdapter struct {
	feeds *feeds.Feeds
}

func (a feedsHealthAdapter) ListUnhealthy(
	ctx context.Context,
) ([]jobs.UnhealthyFeed, error) {
	unhealthy, err := a.feeds.ListUnhealthy(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]jobs.UnhealthyFeed, len(unhealthy))
	for i, feed := range unhealthy {
		out[i] = jobs.UnhealthyFeed{
			Title:               feed.Title,
			URL:                 feed.URL,
			LastError:           feed.LastError,
			ConsecutiveFailures: feed.ConsecutiveFailures,
		}
	}
	return out, nil
}

func newWeeklyDigestJob(
	sentryClient sentryapi.Client,
	githubClient github.Client,
	feedsApp *feeds.Feeds,
	notificationsSvc *notifications.Service,
	notificationSettingsRepo *repositories.NotificationSettingsRepository,
) *jobs.WeeklyDigestJob {
	return jobs.NewWeeklyDigestJob(
		sentryClient, githubClient,
		feedsHealthAdapter{feeds: feedsApp}, notificationsSvc,
		notificationSettingsRepo,
	)
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
	if err := app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.transactionLatencySnapshotJob, app.db),
		noopCallback,
	); err != nil {
		return err
	}
	if err := app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.weeklyDigestJob, app.db), noopCallback,
	); err != nil {
		return err
	}
	if err := app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.hostMetricsSnapshotJob, app.db), noopCallback,
	); err != nil {
		return err
	}
	if err := app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.workflowRunsSnapshotJob, app.db), noopCallback,
	); err != nil {
		return err
	}
	return app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.dbSizeSnapshotJob, app.db), noopCallback,
	)
}

//nolint:funlen //composition root: wiring every dependency, not complex logic
func NewApplication(
	logger *slog.Logger,
	config config.Config,
	db *pgxpool.Pool,
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
	authSealer := newOAuthSealer(logger, config)
	authRepo := auth.NewRepository(db)
	authMailer := mailer.New(
		config.ResendAPIKey,
		config.EmailFrom,
		config.NotifyEmailTo,
	)
	authSvc := auth.NewService(config, authRepo, appUsersRepo, authSealer, authMailer)
	authSvc.SignInRenderer = func(
		w http.ResponseWriter, _ *http.Request, _ string,
	) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}

	contactsSvc, notificationsSvc := newContactsService(
		ctx, logger, config, contactsRepo, authSvc,
	)

	oauthConnRepo := repositories.NewOAuthConnectionsRepository(db, authSealer)
	githubClient, sentryClient := newObservabilityClients(
		logger, config, oauthConnRepo,
	)

	notificationSettingsRepo := repositories.NewNotificationSettingsRepository(db)
	storageSnapshotsRepo := repositories.NewStorageSnapshotsRepository(db)
	issueNotifierJob, transactionLatencyRepo, transactionLatencySnapshotJob :=
		newCrossAppJobs(
			db,
			sentryClient,
			githubClient,
			notificationsSvc,
			notificationSettingsRepo,
			storageSnapshotsRepo,
		)

	hostMetricsRepo := repositories.NewHostMetricsRepository(db)
	logsRepo := repositories.NewLogsRepository(db)
	hostMetricsScraper := observability.NewHostMetricsScraper(
		config.NodeExporterURL, hostMetricsScrapeTimeout,
	)
	hostMetricsSnapshotJob := jobs.NewHostMetricsSnapshotJob(
		hostMetricsScraper, hostMetricsRepo, logsRepo,
	)

	workflowRunsRepo, workflowRunsSnapshotJob := newWorkflowRunsSnapshotJob(
		db, githubClient, notificationsSvc, notificationSettingsRepo,
	)

	dbStatsRepo := repositories.NewDBStatsRepository(db)
	dbSizeSamplesRepo := repositories.NewDBSizeSamplesRepository(db)
	dbSizeSnapshotJob := jobs.NewDBSizeSnapshotJob(dbStatsRepo, dbSizeSamplesRepo)

	//nolint:exhaustruct //apps/booksApp are set after construction, see below
	app := &Application{
		ctx:        ctx,
		logger:     logger,
		config:     config,
		db:         db,
		auth:       authSvc,
		authSealer: authSealer,
		// wired below, after migrations create the auth schema
		oauth2as:                      nil,
		contacts:                      contactsSvc,
		appUsersRepo:                  appUsersRepo,
		profileSharesRepo:             repositories.NewProfileSharesRepository(db),
		usage:                         observability.NewUsageRecorder(logger, db),
		jobRunsRepo:                   repositories.NewJobRunsRepository(db),
		usageRepo:                     repositories.NewUsageRepository(db),
		storageRepo:                   storageSnapshotsRepo,
		dbStatsRepo:                   dbStatsRepo,
		dbSizeSamplesRepo:             dbSizeSamplesRepo,
		dbSizeSnapshotJob:             dbSizeSnapshotJob,
		hostMetricsRepo:               hostMetricsRepo,
		logsRepo:                      logsRepo,
		notificationSettingsRepo:      notificationSettingsRepo,
		oauthConnRepo:                 oauthConnRepo,
		oauthState:                    oauthconn.NewStateStore(),
		githubClient:                  githubClient,
		sentryClient:                  sentryClient,
		issueNotifierJob:              issueNotifierJob,
		transactionLatencyRepo:        transactionLatencyRepo,
		transactionLatencySnapshotJob: transactionLatencySnapshotJob,
		hostMetricsSnapshotJob:        hostMetricsSnapshotJob,
		workflowRunsRepo:              workflowRunsRepo,
		workflowRunsSnapshotJob:       workflowRunsSnapshotJob,
		globalJobQueue: jobqueue.NewJobQueue(
			ctx, logger, globalJobQueueWorkers, globalJobQueueSize, db,
		),
	}

	// One tracing wrapper for every app's queries; migrations keep the raw pool.
	spanDB := postgres.NewSpanDB(db)
	var feedsApp *feeds.Feeds
	app.apps, app.booksApp, feedsApp = NewApps(
		app.auth, logger, config, spanDB, notificationsSvc, appUsersRepo,
	)
	app.feedsApp = feedsApp
	app.weeklyDigestJob = newWeeklyDigestJob(
		sentryClient, githubClient, feedsApp, notificationsSvc,
		notificationSettingsRepo,
	)

	err = app.ApplyMigrations(db)
	if err != nil {
		panic(err)
	}

	oauth2Store := oauth2as.NewStore(spanDB)
	oauth2Provider := oauth2as.NewProvider(config, oauth2Store)
	app.oauth2as = &oauth2asWiring{store: oauth2Store, provider: oauth2Provider}
	app.auth.OAuth2TokenResolver = oauth2as.NewTokenResolver(oauth2Provider)

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
