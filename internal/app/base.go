package app

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

// Base holds the fields and lifecycle helpers shared by every app.
// Embed this struct into your app struct; all fields are exported so
// handler files in the embedding package can access them directly.
type Base struct {
	Logger    *slog.Logger
	Ctx       context.Context
	CtxCancel context.CancelFunc
	Config    config.Config
	Tpl       *template.Template
	Auth      auth.Service
}

// NewBase initialises the shared fields for an app.
// parentCtx is typically context.Background(); pass a derived context when
// the app must inherit cancellation or values from an outer context (e.g. Sentry).
func NewBase(
	parentCtx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	appTemplates embed.FS,
	sharedTpl *template.Template,
) Base {
	tpl := template.Must(sharedTpl.Clone())
	tpl = template.Must(tpl.ParseFS(appTemplates, "templates/html/**/*.html"))

	ctx, cancel := context.WithCancel(parentCtx)

	return Base{
		Logger:    logger,
		Ctx:       ctx,
		CtxCancel: cancel,
		Config:    cfg,
		Tpl:       tpl,
		Auth:      authService,
	}
}

// ApplyMigrationsFromFS runs goose migrations from the given embed.FS under
// a dedicated schema named schemaName.
//
// NOTE: goose uses package-level globals (SetTableName, SetBaseFS, SetDialect).
// This function must not be called concurrently across apps; the existing
// apps.ApplyMigrations loop is sequential and safe.
func (b *Base) ApplyMigrationsFromFS(
	ctx context.Context,
	db *pgxpool.Pool,
	migrations embed.FS,
	schemaName string,
) error {
	if _, err := db.Exec(
		ctx,
		fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schemaName),
	); err != nil {
		return err
	}

	goose.SetTableName(fmt.Sprintf("%s.goose_db_version", schemaName))
	goose.SetLogger(slog.NewLogLogger(b.Logger.Handler(), slog.LevelInfo))
	goose.SetBaseFS(migrations)

	if err := goose.SetDialect(string(goose.DialectPostgres)); err != nil {
		return err
	}

	return goose.Up(stdlib.OpenDBFromPool(db), "migrations")
}
