// Package factory is the admin-only task board of the software factory.
package factory

import (
	"context"
	"embed"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/factory/internal/repositories"
	"tools.xdoubleu.com/apps/factory/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Factory struct {
	app.Base
	services *services.Services
	// stepUp checks the TOTP code that creating a runner needs.
	stepUp        StepUp
	heartbeatHold time.Duration
}

// Option adjusts a Factory.
type Option func(*Factory)

// WithStepUp sets the check run before a runner is created. Without it,
// creating a runner is refused.
func WithStepUp(stepUp StepUp) Option {
	return func(a *Factory) { a.stepUp = stepUp }
}

// WithHeartbeatHold sets how long a Heartbeat is held open; tests shorten it.
func WithHeartbeatHold(d time.Duration) Option {
	return func(a *Factory) { a.heartbeatHold = d }
}

func New(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	opts ...Option,
) *Factory {
	a := &Factory{
		Base:          app.NewBase(ctx, authService, logger, cfg),
		services:      services.New(repositories.New(db), authService),
		stepUp:        nil,
		heartbeatHold: DefaultHeartbeatHold,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func (a *Factory) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	return a.ApplyMigrationsFromFS(ctx, db, embedMigrations, a.GetName())
}

func (a *Factory) GetName() string {
	return "factory"
}

func (a *Factory) GetDisplayName() string {
	return "Factory"
}

func (a *Factory) Start() error {
	return nil
}
