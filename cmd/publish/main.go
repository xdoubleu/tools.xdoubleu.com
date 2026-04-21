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
	"github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	essentialogger "github.com/xdoubleu/essentia/v3/pkg/logging"
	"github.com/xdoubleu/essentia/v3/pkg/sentrytools"
	goaltracker "tools.xdoubleu.com/apps/goaltracker"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
	"tools.xdoubleu.com/cmd/publish/internal/services"
	"tools.xdoubleu.com/internal/config"
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
	apps          *Apps
	goalTracker   *goaltracker.GoalTracker
	tpl           *template.Template
	requestBuffer *logging.UserLogBuffer
	appUsersRepo  *repositories.AppUsersRepository
}

//	@title			tools
//	@version		1.0
//	@license.name	GPL-3.0
//	@Accept			json
//	@Produce		json

func main() {
	cfg := config.New(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	logger := slog.New(sentrytools.NewLogHandler(cfg.Env,
		slog.NewTextHandler(os.Stdout, nil)))
	db, err := postgres.Connect(
		logger,
		cfg.DBDsn,
		25, //nolint:mnd //no magic number
		"15m",
		60,             //nolint:mnd //no magic number
		10*time.Second, //nolint:mnd //no magic number
		5*time.Minute,  //nolint:mnd //no magic number
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
		ReadTimeout:  5 * time.Second,  //nolint:mnd //no magic number
		WriteTimeout: 10 * time.Second, //nolint:mnd //no magic number
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
	})
	tpl = template.Must(tpl.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	ctx := context.Background()

	sentryHub := initSentryGetHub(config)
	if sentryHub != nil {
		ctx = sentry.SetHubOnContext(ctx, sentryHub)
	}

	//nolint:mnd // 100 log entries per user
	logBuffer := logging.NewUserLogBuffer(100)

	appUsersRepo := repositories.NewAppUsersRepository(db)
	svc := services.New(config, supabaseClient, tpl, appUsersRepo)

	gt := goaltracker.New(ctx, svc.Auth, logger, config, db, sharedTpl)

	//nolint:exhaustruct //other fields are optional
	app := &Application{
		ctx:           ctx,
		logger:        slog.New(logging.NewUserLogHandler(logger.Handler(), logBuffer)),
		config:        config,
		services:      svc,
		goalTracker:   gt,
		tpl:           tpl,
		requestBuffer: logBuffer,
		appUsersRepo:  appUsersRepo,
	}

	app.apps = NewApps(app.ctx, app.services.Auth, logger, config, db, sharedTpl, gt)

	err := app.ApplyMigrations(db)
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

func initSentryGetHub(config config.Config) *sentry.Hub {
	if len(config.SentryDsn) == 0 {
		return nil
	}

	//nolint:exhaustruct //other fields are optional
	sentryClientOptions := sentry.ClientOptions{
		Dsn:              config.SentryDsn,
		Environment:      config.Env,
		Release:          config.Release,
		EnableTracing:    true,
		TracesSampleRate: config.SampleRate,
		SampleRate:       config.SampleRate,
	}

	err := sentry.Init(sentryClientOptions)

	if err != nil {
		panic(err)
	}

	return sentry.CurrentHub().Clone()
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
