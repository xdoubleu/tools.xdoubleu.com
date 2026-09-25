package app

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

// Base holds the fields and lifecycle helpers shared by every app; embed it.
type Base struct {
	Logger    *slog.Logger
	Ctx       context.Context
	CtxCancel context.CancelFunc
	Config    config.Config
	Auth      auth.Service
}

// NewBase initialises the shared fields for an app.
func NewBase(
	parentCtx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
) Base {
	ctx, cancel := context.WithCancel(parentCtx)

	return Base{
		Logger:    logger,
		Ctx:       ctx,
		CtxCancel: cancel,
		Config:    cfg,
		Auth:      authService,
	}
}

func (b *Base) GetDisplayName() string { return "" }

func (b *Base) GetDomain() string { return "" }

// ApplyMigrationsFromFS runs goose migrations into schemaName. goose uses
// package globals, so it must never run concurrently across apps.
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

	return goose.Up(stdlib.OpenDBFromPool(db), "migrations", goose.WithAllowMissing())
}
