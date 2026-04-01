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
	"github.com/supabase-community/gotrue-go"
	"github.com/xdoubleu/essentia/v3/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v3/pkg/logging"
	"github.com/xdoubleu/essentia/v3/pkg/sentrytools"
	"tools.xdoubleu.com/cmd/publish/internal/services"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/templates"
)

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

type Application struct {
	ctx      context.Context
	logger   *slog.Logger
	config   config.Config
	services *services.Services
	apps     *Apps
	tpl      *template.Template
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
		logger.Error("failed to serve server", logging.ErrAttr(err))
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
	tpl = template.Must(tpl.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	ctx := context.Background()

	sentryHub := initSentryGetHub(config)
	if sentryHub != nil {
		ctx = sentry.SetHubOnContext(ctx, sentryHub)
	}

	//nolint:exhaustruct //other fields are optional
	app := &Application{
		ctx:      ctx,
		logger:   logger,
		config:   config,
		services: services.New(config, supabaseClient, tpl),
		tpl:      tpl,
	}

	apps := NewApps(app.ctx, app.services.Auth, logger, config, db, sharedTpl)

	err := apps.ApplyMigrations(db)
	if err != nil {
		panic(err)
	}

	app.apps = apps

	for _, app := range apps.apps {
		err = app.Start()
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
	return app.apps.ApplyMigrations(db)
}
