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
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

//go:embed templates/html/**/*html
var htmlTemplates embed.FS

type WatchParty struct {
	logger    *slog.Logger
	ctx       context.Context
	ctxCancel context.CancelFunc
	config    config.Config
	Services  *services.Services
	tpl       *template.Template
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	sharedTpl *template.Template,
) *WatchParty {
	tpl := template.Must(sharedTpl.Clone())
	tpl = template.Must(tpl.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	//nolint:exhaustruct //other fields are optional
	app := &WatchParty{
		logger: logger,
		config: cfg,
		tpl:    tpl,
	}

	app.setContext()
	app.Services = services.New(app.ctx, logger, authService)

	return app
}

func (app *WatchParty) ApplyMigrations(_ context.Context, _ *pgxpool.Pool) error {
	return nil
}

func (app *WatchParty) Start() error {
	return nil
}

func (app *WatchParty) setContext() {
	//nolint:gosec //cancel called later
	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	app.ctxCancel = cancel
}

func (app *WatchParty) GetName() string {
	return "watchparty"
}
