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
	"tools.xdoubleu.com/internal/account"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/crypto"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/family"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/jobqueue"
	essentialogger "tools.xdoubleu.com/internal/logging"
	"tools.xdoubleu.com/internal/mailer"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/obs"
	"tools.xdoubleu.com/internal/observability"
	"tools.xdoubleu.com/internal/observability/jobs"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/sentryapi"
	"tools.xdoubleu.com/internal/sentrytools"
)

//go:embed migrations/*.sql
var globalMigrations embed.FS

// storageScanRunner is the slice of *books.Books TriggerStorageScan needs.
type storageScanRunner interface {
	StartStorageScan()
}

// unhealthyFeedLister is the slice of *feeds.Feeds GetUnhealthyFeeds needs.
// Unlike jobs.unhealthyFeedLister, it returns feeds.UnhealthyFeed directly.
type unhealthyFeedLister interface {
	ListUnhealthy(ctx context.Context) ([]feeds.UnhealthyFeed, error)
}

type Application struct {
	ctx                           context.Context
	logger                        *slog.Logger
	config                        config.Config
	db                            *pgxpool.Pool
	auth                          *auth.LocalService
	authSealer                    *crypto.Sealer
	oauth2as                      *oauth2asWiring
	family                        family.Service
	apps                          *Apps
	booksApp                      storageScanRunner
	feedsApp                      unhealthyFeedLister
	appUsersRepo                  *repositories.AppUsersRepository
	profileSharesRepo             *repositories.ProfileSharesRepository
	usage                         *observability.UsageRecorder
	notifications                 *notifications.Service
	jobRunsRepo                   *repositories.JobRunsRepository
	usageRepo                     *repositories.UsageRepository
	storageRepo                   *repositories.StorageSnapshotsRepository
	dbStatsRepo                   *repositories.DBStatsRepository
	automatedActionsRepo          *repositories.AutomatedActionsRepository
	logsRepo                      *repositories.LogsRepository
	notificationSettingsRepo      *repositories.NotificationSettingsRepository
	githubClient                  github.Client
	sentryClient                  sentryapi.Client
	oauthConnRepo                 *repositories.OAuthConnectionsRepository
	oauthState                    *oauthconn.StateStore
	issueSignalCollectorJob       *jobs.IssueSignalCollectorJob
	automatedActionSweepJob       *jobs.AutomatedActionSweepJob
	transactionLatencyRepo        *repositories.TransactionLatencyRepository
	transactionLatencySnapshotJob *jobs.TransactionLatencySnapshotJob
	weeklyDigestJob               *jobs.WeeklyDigestJob
	logPruneJob                   *jobs.LogPruneJob
	globalJobQueue                *jobqueue.JobQueue
}

// accountDeps builds the account module's Deps from the wiring Application
// already holds.
func (app *Application) accountDeps() *account.Deps {
	return &account.Deps{
		Auth:     app.auth,
		Config:   app.config,
		AppUsers: app.appUsersRepo,
		Family:   app.family,
		Shares:   app.profileSharesRepo,
	}
}

// obsDeps builds the observability module's Deps from the wiring Application
// already holds. booksApp is set after construction, so this is a method, not
// a struct field.
func (app *Application) obsDeps() *obs.Deps {
	appNames := make([]string, 0, len(*app.apps))
	for _, a := range *app.apps {
		appNames = append(appNames, a.GetName())
	}

	return &obs.Deps{
		Logger:   app.logger,
		Config:   app.config,
		AppUsersRepo: app.appUsersRepo,
		JobRunsRepo:  app.jobRunsRepo,
		UsageRepo:    app.usageRepo,
		StorageRepo:  app.storageRepo,
		DBStatsRepo:  app.dbStatsRepo,
		AutomatedActionsRepo: app.automatedActionsRepo,
		LogsRepo:             app.logsRepo,
		NotificationSettingsRepo: app.notificationSettingsRepo,
		OAuthConnRepo:    app.oauthConnRepo,
		TransactionLatencyRepo: app.transactionLatencyRepo,
		OAuthState:   app.oauthState,
		GithubClient: app.githubClient,
		SentryClient: app.sentryClient,
		StorageScan: app.booksApp,
		AppNames:    appNames,
	}
}

//	@title			tools
//	@version		1.0
//	@license.name	GPL-3.0
//	@Accept			json
//	@Produce		json

const (
	dbMaxIdleTime        = "15m"
	dbConnectTimeoutSecs = 10
	dbRetrySleep         = 2 * time.Second
	dbMaxRetryDuration   = 20 * time.Second
	httpReadTimeout      = 5 * time.Second
	httpWriteTimeout     = 10 * time.Second
	// migrationLockKey is the advisory lock serializing migrations across
	// replicas.
	migrationLockKey = 20260101
	// usageFlushInterval is how often request counts go to global.usage_daily.
	usageFlushInterval = time.Minute
	// globalJobQueueWorkers/globalJobQueueSize size the cross-app job queue.
	globalJobQueueWorkers = 1
	globalJobQueueSize    = 10
	// The heap watchdog thresholds sit below each role's container cap
	// (config/deploy.api.yml) so it logs before the kill; the workers cap is
	// above the memory-heavy books jobs' peak. See observability.WatchHeap.
	webHeapWatchThreshold     = 400 << 20
	workersHeapWatchThreshold = 1200 << 20
	heapWatchInterval         = 5 * time.Second
	logFlushTimeout           = 5 * time.Second
)

// migrationLockTimeout makes a replica fail loudly if a stale connection
// holds the migration lock. A var so tests can shrink it.
//
//nolint:gochecknoglobals //test seam, see comment above
var migrationLockTimeout = 20 * time.Second

// newDBPool opens the shared pgx pool; tests reuse it.
func newDBPool(
	logger *slog.Logger,
	dsn string,
	maxConns int,
) (*pgxpool.Pool, error) {
	return postgres.Connect(
		logger, dsn, maxConns, dbMaxIdleTime,
		dbConnectTimeoutSecs, dbRetrySleep, dbMaxRetryDuration,
	)
}

func main() {
	ctx, stop := newRootContext()
	defer stop()

	cfg := config.New(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	bootLogger := slog.New(sentrytools.NewLogHandler(cfg.Env,
		slog.NewTextHandler(os.Stdout, nil)))
	db, err := newDBPool(bootLogger, cfg.DBDsn, cfg.DBMaxConns)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Built after the DB pool, which the log-teeing handler needs.
	logHandler := observability.NewLogRepoHandler(
		sentrytools.NewLogHandler(cfg.Env, slog.NewTextHandler(os.Stdout, nil)),
		repositories.NewLogsRepository(db),
	)
	// Deferred after db.Close, so it flushes first.
	defer flushLogs(logHandler)
	logger := slog.New(logHandler)
	// Route slog.Default() through the same chain.
	slog.SetDefault(logger)

	app := NewApplication(ctx, logger, cfg, db)

	go observability.WatchHeap(
		ctx, logger, heapWatchThreshold(cfg.Role), heapWatchInterval,
	)

	handler, err := app.Handler()
	if err != nil {
		panic(err)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		IdleTimeout:  time.Minute,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
	}
	err = httptools.Serve(ctx, logger, srv, cfg.Env, httpShutdownTimeout)
	if err != nil {
		logger.Error("failed to serve server", essentialogger.ErrAttr(err))
	}

	// Drain background tasks, job pools, email and usage counts here; the
	// deferred log flush and db.Close then run, in that order. The flush stays
	// deferred so a panicking boot still writes its logs.
	stop()
	if err = app.Shutdown(context.Background()); err != nil {
		logger.ErrorContext(app.ctx, "shutdown did not drain cleanly",
			essentialogger.ErrAttr(err))
	}
	sentry.Flush(sentryFlushTimeout)
}

// flushLogs writes the log records still queued for global.log_entries.
func flushLogs(logHandler *observability.LogRepoHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), logFlushTimeout)
	defer cancel()
	_ = logHandler.Close(ctx)
}

func heapWatchThreshold(role string) uint64 {
	if role == config.RoleWorkers {
		return workersHeapWatchThreshold
	}
	return webHeapWatchThreshold
}

// handler is the full API for the web role; the workers role serves only
// /health and /metrics, which Prometheus scrapes by the shared service label.
func (app *Application) handler() http.Handler {
	if !app.config.IsWorkers() {
		return app.Routes()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+healthPath, app.healthHandler)
	mux.Handle("GET "+metricsPath, metricsHandler(app.config.ObservabilityIngestSecret))
	return mux
}

// newOAuthSealer builds the AES-GCM sealer for OAuth tokens and TOTP secrets.
// Returns nil without ENCRYPTION_KEY; those features are then unavailable.
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

// newNotificationsService builds the shared Resend-backed FIFO notifications
// queue, so mail never blocks a request.
func newNotificationsService(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
) *notifications.Service {
	mailClient := mailer.New(
		config.ResendAPIKey,
		config.EmailFrom,
		config.NotifyEmailTo,
	)
	return notifications.New(ctx, logger, mailClient)
}

// newObservabilityClients builds the external observability clients, each
// resolving its token from oauthConnRepo.
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

// newCrossAppJobs builds the jobs on globalJobQueue. transactionLatencyRepo is
// returned too, since handlers read it directly.
func newCrossAppJobs(
	db *pgxpool.Pool,
	sentryClient sentryapi.Client,
	githubClient github.Client,
	storageSnapshotsRepo *repositories.StorageSnapshotsRepository,
	dbStatsRepo *repositories.DBStatsRepository,
	automatedActionsRepo *repositories.AutomatedActionsRepository,
) (
	*jobs.IssueSignalCollectorJob,
	*repositories.TransactionLatencyRepository,
	*jobs.TransactionLatencySnapshotJob,
) {
	transactionLatencyRepo := repositories.NewTransactionLatencyRepository(db)

	issueSignalCollectorJob := jobs.NewIssueSignalCollectorJob(
		githubClient, sentryClient, storageSnapshotsRepo, dbStatsRepo,
		automatedActionsRepo, transactionLatencyRepo,
	)

	transactionLatencySnapshotJob := jobs.NewTransactionLatencySnapshotJob(
		sentryClient, transactionLatencyRepo,
	)

	return issueSignalCollectorJob, transactionLatencyRepo,
		transactionLatencySnapshotJob
}

// feedsHealthAdapter bridges feeds.UnhealthyFeed to jobs.UnhealthyFeed so the
// jobs package never imports apps/feeds.
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

// feedsOpenItemsAdapter bridges *feeds.Feeds to jobs.openFeedItemsLister.
type feedsOpenItemsAdapter struct {
	feeds *feeds.Feeds
}

func (a feedsOpenItemsAdapter) ListOpenItems(
	ctx context.Context,
) ([]jobs.OpenFeedItem, error) {
	open, err := a.feeds.ListOpenItems(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]jobs.OpenFeedItem, len(open))
	for i, feed := range open {
		out[i] = jobs.OpenFeedItem{
			Title: feed.Title,
			URL:   feed.URL,
			Count: feed.Count,
		}
	}
	return out, nil
}

func newWeeklyDigestJob(
	feedsApp *feeds.Feeds,
	notificationsSvc *notifications.Service,
	notificationSettingsRepo *repositories.NotificationSettingsRepository,
) *jobs.WeeklyDigestJob {
	return jobs.NewWeeklyDigestJob(
		feedsHealthAdapter{feeds: feedsApp},
		feedsOpenItemsAdapter{feeds: feedsApp},
		notificationsSvc,
		notificationSettingsRepo,
	)
}

// startCrossAppJobs registers the jobs on app.globalJobQueue.
func startCrossAppJobs(app *Application) error {
	noopCallback := func(_ string, _ bool, _ *time.Time) {}
	if err := app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.issueSignalCollectorJob, app.db), noopCallback,
	); err != nil {
		return err
	}
	if err := app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.automatedActionSweepJob, app.db),
		noopCallback,
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
	return app.globalJobQueue.AddJob(
		observability.NewTrackedJob(app.logPruneJob, app.db), noopCallback,
	)
}

//nolint:funlen //composition root: wiring every dependency, not complex logic
func NewApplication(
	ctx context.Context,
	logger *slog.Logger,
	config config.Config,
	db *pgxpool.Pool,
) *Application {
	sentryHub, err := sentrytools.Init(config.Env, sentry.ClientOptions{
		Dsn:                   config.SentryDsn,
		Environment:           config.Env,
		Release:               config.Release,
		EnableTracing:         true,
		TracesSampleRate:      config.SampleRate,
		SampleRate:            config.SampleRate,
		BeforeSend:            scrubSentryEvent,
		BeforeSendTransaction: scrubSentryEvent,
	})
	if err != nil {
		panic(err)
	}
	if sentryHub != nil {
		ctx = sentry.SetHubOnContext(ctx, sentryHub)
	}

	appUsersRepo := repositories.NewAppUsersRepository(db)
	familyRepo := repositories.NewFamilyRepository(db)
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

	notificationsSvc := newNotificationsService(ctx, logger, config)
	familySvc := family.New(
		familyRepo,
		authSvc,
		notificationsSvc,
		config.WebURL,
		logger,
	)

	oauthConnRepo := repositories.NewOAuthConnectionsRepository(db, authSealer)
	githubClient, sentryClient := newObservabilityClients(
		logger, config, oauthConnRepo,
	)

	notificationSettingsRepo := repositories.NewNotificationSettingsRepository(db)
	storageSnapshotsRepo := repositories.NewStorageSnapshotsRepository(db)
	dbStatsRepo := repositories.NewDBStatsRepository(db)
	automatedActionsRepo := repositories.NewAutomatedActionsRepository(db)
	automatedActionSweepJob := jobs.NewAutomatedActionSweepJob(
		automatedActionsRepo,
	)
	issueSignalCollectorJob, transactionLatencyRepo,
		transactionLatencySnapshotJob := newCrossAppJobs(
		db,
		sentryClient,
		githubClient,
		storageSnapshotsRepo,
		dbStatsRepo,
		automatedActionsRepo,
	)

	logsRepo := repositories.NewLogsRepository(db)

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
		family:                        familySvc,
		appUsersRepo:                  appUsersRepo,
		profileSharesRepo:             repositories.NewProfileSharesRepository(db),
		usage:                         observability.NewUsageRecorder(logger, db),
		notifications:                 notificationsSvc,
		jobRunsRepo:                   repositories.NewJobRunsRepository(db),
		usageRepo:                     repositories.NewUsageRepository(db),
		storageRepo:                   storageSnapshotsRepo,
		dbStatsRepo:                   dbStatsRepo,
		automatedActionsRepo:          automatedActionsRepo,
		logsRepo:                      logsRepo,
		notificationSettingsRepo:      notificationSettingsRepo,
		oauthConnRepo:                 oauthConnRepo,
		oauthState:                    oauthconn.NewStateStore(),
		githubClient:                  githubClient,
		sentryClient:                  sentryClient,
		issueSignalCollectorJob:       issueSignalCollectorJob,
		automatedActionSweepJob:       automatedActionSweepJob,
		transactionLatencyRepo:        transactionLatencyRepo,
		transactionLatencySnapshotJob: transactionLatencySnapshotJob,
		globalJobQueue: jobqueue.NewJobQueue(
			ctx, logger, globalJobQueueWorkers, globalJobQueueSize, db,
		),
	}

	// One tracing wrapper for every app's queries; migrations keep the raw pool.
	spanDB := postgres.NewSpanDB(db)
	var feedsApp *feeds.Feeds
	app.apps, app.booksApp, feedsApp = NewApps(
		ctx, app.auth, logger, config, spanDB, notificationsSvc, appUsersRepo,
		familyRepo, authSealer,
	)
	app.feedsApp = feedsApp
	app.weeklyDigestJob = newWeeklyDigestJob(
		feedsApp, notificationsSvc, notificationSettingsRepo,
	)
	app.logPruneJob = jobs.NewLogPruneJob(logsRepo)

	err = app.ApplyMigrations(db)
	if err != nil {
		panic(err)
	}

	oauth2Store := oauth2as.NewStore(spanDB)
	oidcKey := oauth2as.LoadOIDCKeyOrDegrade(logger, config.OAuthOIDCPrivateKey)
	oauth2Provider := oauth2as.NewProvider(config, oauth2Store, oidcKey)
	app.oauth2as = &oauth2asWiring{
		store: oauth2Store, provider: oauth2Provider, oidcKey: oidcKey,
	}
	app.auth.OAuth2TokenResolver = oauth2as.NewTokenResolver(oauth2Provider)

	if err = oauth2as.EnsureGrafanaClientSecret(
		ctx, spanDB, config.OAuthGrafanaClientSecret, logger,
	); err != nil {
		panic(err)
	}
	if err = oauth2as.EnsureRoutinesClientSecret(
		ctx, spanDB, config.OAuthRoutinesClientSecret, logger,
	); err != nil {
		panic(err)
	}

	if !config.IsWorkers() {
		// Flush request counts for the process lifetime.
		app.usage.Start(ctx, usageFlushInterval)

		if err = startCrossAppJobs(app); err != nil {
			panic(err)
		}
	}

	if err = app.apps.Start(config.Role); err != nil {
		panic(err)
	}

	return app
}

func (app *Application) ApplyMigrations(db *pgxpool.Pool) error {
	// Session-level advisory lock on a dedicated connection so replicas never
	// migrate concurrently.
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
