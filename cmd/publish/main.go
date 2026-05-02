package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/supabase-community/gotrue-go"
	"github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
	"github.com/xdoubleu/essentia/v4/pkg/sentrytools"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
	"tools.xdoubleu.com/cmd/publish/internal/services"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/contacts"
	"tools.xdoubleu.com/internal/repositories"
	"tools.xdoubleu.com/internal/templates"
)

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

//go:embed migrations/*.sql
var globalMigrations embed.FS

type Application struct {
	ctx           context.Context
	logger        *slog.Logger
	config        config.Config
	services      *services.Services
	contacts      contacts.Service
	apps          *Apps
	backlog       *backlog.Backlog
	tpl           *template.Template
	requestBuffer *logging.UserLogBuffer
	appUsersRepo  *repositories.AppUsersRepository
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
	userLogBufSize   = 100
)

func main() {
	cfg := config.New(slog.New(slog.NewTextHandler(os.Stdout, nil)))

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

func NewApplication(
	logger *slog.Logger,
	config config.Config,
	db *pgxpool.Pool,
	supabaseClient gotrue.Client,
) *Application {
	sharedTpl := templates.LoadShared(config)
	tpl := template.Must(sharedTpl.Clone())
	tpl = tpl.Funcs(template.FuncMap{
		"hasAccess": func(access []string, appName string) bool {
			for _, a := range access {
				if a == appName {
					return true
				}
			}
			return false
		},
		"dict": func(kv ...any) map[string]any {
			const kvPairSize = 2
			m := make(map[string]any, len(kv)/kvPairSize)
			for i := 0; i+1 < len(kv); i += kvPairSize {
				if key, ok := kv[i].(string); ok {
					m[key] = kv[i+1]
				}
			}
			return m
		},
	})
	tpl = template.Must(tpl.ParseFS(htmlTemplates, "templates/html/**/*.html"))

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

	logBuffer := logging.NewUserLogBuffer(userLogBufSize)

	appUsersRepo := repositories.NewAppUsersRepository(db)
	contactsRepo := repositories.NewContactsRepository(db)
	svc := services.New(config, supabaseClient, tpl, appUsersRepo)
	contactsSvc := contacts.New(contactsRepo, svc.Auth)

	bl := backlog.New(ctx, svc.Auth, logger, config, db, sharedTpl)

	//nolint:exhaustruct //other fields are optional
	app := &Application{
		ctx:           ctx,
		logger:        slog.New(logging.NewUserLogHandler(logger.Handler(), logBuffer)),
		config:        config,
		services:      svc,
		contacts:      contactsSvc,
		backlog:       bl,
		tpl:           tpl,
		requestBuffer: logBuffer,
		appUsersRepo:  appUsersRepo,
	}

	app.apps = NewApps(
		app.ctx, app.services.Auth, logger, config, db, sharedTpl, bl, contactsSvc,
	)

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
	if err := app.applyGlobalMigrations(db); err != nil {
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
