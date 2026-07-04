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
	auth "github.com/supabase-community/auth-go"
	"github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
	"github.com/xdoubleu/essentia/v4/pkg/sentrytools"

	"tools.xdoubleu.com/cmd/api/internal/services"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/contacts"
	"tools.xdoubleu.com/internal/repositories"
)

//go:embed migrations/*.sql
var globalMigrations embed.FS

//nolint:gochecknoglobals //Release is set at build time via -ldflags.
var Release = "dev"

type Application struct {
	ctx          context.Context
	logger       *slog.Logger
	config       config.Config
	db           *pgxpool.Pool
	services     *services.Services
	contacts     contacts.Service
	apps         *Apps
	appUsersRepo *repositories.AppUsersRepository
}

//	@title			tools
//	@version		1.0
//	@license.name	GPL-3.0
//	@Accept			json
//	@Produce		json

const (
	dbMaxConns       = 25
	dbMaxIdleTime    = "15m"
	dbMaxLifetime    = 60
	dbConnectTimeout = 10 * time.Second
	dbHealthCheck    = 5 * time.Minute
	httpReadTimeout  = 5 * time.Second
	httpWriteTimeout = 10 * time.Second
	// migrationLockKey identifies the advisory lock that serializes
	// migration runs across concurrently starting replicas.
	migrationLockKey = 20260101
)

func main() {
	cfg := config.New(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	// Release is set at build time via -ldflags; always use that value
	cfg.Release = Release

	logger := slog.New(sentrytools.NewLogHandler(cfg.Env,
		slog.NewTextHandler(os.Stdout, nil)))
	db, err := postgres.Connect(
		logger,
		cfg.DBDsn,
		dbMaxConns,
		dbMaxIdleTime,
		dbMaxLifetime,
		dbConnectTimeout,
		dbHealthCheck,
	)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	supabase := auth.New(
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

func NewApplication(
	logger *slog.Logger,
	config config.Config,
	db *pgxpool.Pool,
	supabaseClient auth.Client,
) *Application {
	ctx := context.Background()

	//nolint:exhaustruct //other fields are optional
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
	svc := services.New(config, supabaseClient, appUsersRepo)
	svc.Auth.SignInRenderer = func(
		w http.ResponseWriter, _ *http.Request, _ string,
	) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
	contactsSvc := contacts.New(contactsRepo, svc.Auth)

	//nolint:exhaustruct //other fields are optional
	app := &Application{
		ctx:          ctx,
		logger:       logger,
		config:       config,
		db:           db,
		services:     svc,
		contacts:     contactsSvc,
		appUsersRepo: appUsersRepo,
	}

	// One tracing wrapper for every app's queries; migrations keep the raw pool.
	spanDB := postgres.NewSpanDB(db)
	app.apps = NewApps(app.services.Auth, logger, config, spanDB)

	err = app.ApplyMigrations(db)
	if err != nil {
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

	if _, err = lockConn.Exec(
		app.ctx, "SELECT pg_advisory_lock($1)", migrationLockKey,
	); err != nil {
		return err
	}
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
