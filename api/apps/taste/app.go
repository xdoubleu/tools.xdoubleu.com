// Package taste builds a per-user consumption log across every media app and
// its cross-domain statistics. It reaches other apps only through their
// exported ListConsumption methods.
package taste

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/taste/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

// Source is an app exporting its consumed items.
type Source = services.Source

type Taste struct {
	app.Base
	auth auth.Service
	log  *services.LogService
}

// New builds the app over sources, which must already be constructed
// (registration order in cmd/api/apps.go).
func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	sources ...Source,
) *Taste {
	return &Taste{
		Base: app.NewBase(context.Background(), authService, logger, cfg),
		auth: authService,
		log:  services.NewLogService(sources),
	}
}

func (a *Taste) ApplyMigrations(_ context.Context, _ *pgxpool.Pool) error {
	return nil
}

func (a *Taste) Start() error {
	return nil
}

func (a *Taste) GetName() string {
	return "taste"
}

func (a *Taste) GetDisplayName() string {
	return "Taste"
}
