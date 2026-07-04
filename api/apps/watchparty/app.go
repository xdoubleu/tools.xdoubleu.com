//nolint:revive //it is what it is
package watchparty

import (
	"context"
	"log/slog"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"

	"tools.xdoubleu.com/apps/watchparty/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

type WatchParty struct {
	app.Base
	// Services is exported so integration tests can seed rooms through the
	// real service layer (same convention as the games and books apps).
	Services *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
) *WatchParty {
	//nolint:exhaustruct //Services initialised below
	wp := &WatchParty{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
		),
	}
	wp.Services = services.New(wp.Ctx, logger, authService)

	return wp
}

func (app *WatchParty) ApplyMigrations(_ context.Context, _ *pgxpool.Pool) error {
	return nil
}

func (app *WatchParty) Start() error {
	return nil
}

func (app *WatchParty) GetName() string {
	return "watchparty"
}

func (app *WatchParty) GetDisplayName() string {
	return "WatchParty"
}

func (app *WatchParty) GetDomain() string {
	return "watchparty.xdoubleu.com"
}
