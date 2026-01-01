package watchparty

import (
	"context"
	"embed"
	"html/template"
	"log/slog"
	// needed for embedding timezone data.
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
	services  *services.Services
	tpl       *template.Template
}

func New(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
) *WatchParty {
	tpl := template.Must(template.ParseFS(htmlTemplates, "templates/html/**/*.html"))

	//nolint:exhaustruct //other fields are optional
	app := &WatchParty{
		logger:   logger,
		config:   cfg,
		tpl:      tpl,
		services: services.New(logger, authService),
	}

	app.setContext()

	return app
}

func (app *WatchParty) ApplyMigrations(_ *pgxpool.Pool) error {
	return nil
}

func (app *WatchParty) setContext() {
	ctx, cancel := context.WithCancel(context.Background())
	app.ctx = ctx
	app.ctxCancel = cancel
}

func (app *WatchParty) GetName() string {
	return "watchparty"
}
