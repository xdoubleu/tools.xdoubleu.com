package icsproxy

import (
	"context"
	"embed"
	"html/template"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/xdoubleu/essentia/v2/pkg/database/postgres"
	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
	"tools.xdoubleu.com/apps/icsproxy/internal/services"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

type ICSProxy struct {
	logger    *slog.Logger
	ctx       context.Context
	ctxCancel context.CancelFunc
	config    config.Config
	tpl       *template.Template
	services  *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *ICSProxy {
	tpl := template.Must(template.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	//nolint:exhaustruct //other fields are optional
	app := &ICSProxy{
		logger:   logger,
		config:   cfg,
		tpl:      tpl,
		services: services.New(repositories.New(db), authService),
	}

	app.setContext()

	return app
}

func (app *ICSProxy) ApplyMigrations(db *pgxpool.Pool) error {
	migrationsDB := stdlib.OpenDBFromPool(db)

	goose.SetLogger(slog.NewLogLogger(app.logger.Handler(), slog.LevelInfo))

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect(string(goose.DialectPostgres)); err != nil {
		return err
	}

	if err := goose.Up(migrationsDB, "migrations"); err != nil {
		return err
	}

	return nil
}

func (app *ICSProxy) setContext() {
	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	app.ctxCancel = cancel
}

func (app *ICSProxy) GetName() string {
	return "icsproxy"
}
