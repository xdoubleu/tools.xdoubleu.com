package icsproxy

import (
	"context"
	"embed"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/icsproxy/internal/repositories"
	"tools.xdoubleu.com/apps/icsproxy/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type ICSProxy struct {
	app.Base
	services *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *ICSProxy {
	//nolint:exhaustruct //services initialised below
	proxy := &ICSProxy{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
		),
	}
	proxy.services = services.New(
		logger,
		repositories.New(db),
		authService,
	)

	return proxy
}

func (app *ICSProxy) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return app.ApplyMigrationsFromFS(ctx, db, embedMigrations, app.GetName())
}

func (app *ICSProxy) Start() error {
	return nil
}

func (app *ICSProxy) GetName() string {
	return "icsproxy"
}

func (app *ICSProxy) GetDisplayName() string {
	return "ICS Proxy"
}
