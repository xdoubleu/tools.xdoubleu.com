package main

import (
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/supabase-community/gotrue-go"
	"github.com/xdoubleu/essentia/v2/pkg/communication/httptools"
	"github.com/xdoubleu/essentia/v2/pkg/database/postgres"
	"github.com/xdoubleu/essentia/v2/pkg/logging"
	"github.com/xdoubleu/essentia/v2/pkg/sentrytools"
	"tools.xdoubleu.com/cmd/publish/internal/services"
	"tools.xdoubleu.com/internal/config"
)

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

type Application struct {
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
	tpl := template.Must(template.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	//nolint:exhaustruct //other fields are optional
	app := &Application{
		logger:   logger,
		config:   config,
		services: services.New(config, supabaseClient, tpl),
		tpl:      tpl,
	}

	apps := NewApps(app.services.Auth, logger, config, db)

	err := apps.ApplyMigrations(db)
	if err != nil {
		panic(err)
	}

	app.apps = apps

	return app
}

func (app *Application) ApplyMigrations(db *pgxpool.Pool) error {
	return app.apps.ApplyMigrations(db)
}
