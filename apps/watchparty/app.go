//nolint:revive //it is what it is
package watchparty

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	"tools.xdoubleu.com/apps/watchparty/internal/services"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

type WatchParty struct {
	app.Base
	Services *services.Services
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	sharedTpl *template.Template,
) *WatchParty {
	//nolint:exhaustruct //Services initialised below
	wp := &WatchParty{
		Base: app.NewBase(
			context.Background(),
			authService,
			logger,
			cfg,
			htmlTemplates,
			sharedTpl,
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
