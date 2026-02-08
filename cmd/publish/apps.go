package main

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v2/pkg/database/postgres"
	goaltracker "tools.xdoubleu.com/apps/goaltracker"
	"tools.xdoubleu.com/apps/icsproxy"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

type Apps struct {
	apps []App
}

type App interface {
	Routes(prefix string, mux *http.ServeMux)
	ApplyMigrations(db *pgxpool.Pool) error
	GetName() string
}

func NewApps(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) *Apps {
	apps := &Apps{
		apps: []App{},
	}

	apps.addApp(goaltracker.New(authService, logger, cfg, db))
	apps.addApp(watchparty.New(authService, logger, cfg))
	apps.addApp(icsproxy.New(authService, logger, cfg, db))

	return apps
}

func (apps *Apps) ApplyMigrations(db *pgxpool.Pool) error {
	for _, app := range apps.apps {
		err := app.ApplyMigrations(db)
		if err != nil {
			return err
		}
	}
	return nil
}

func (apps *Apps) Routes(mux *http.ServeMux) http.Handler {
	for _, app := range apps.apps {
		app.Routes(app.GetName(), mux)
	}
	return mux
}

func (apps *Apps) addApp(app App) {
	apps.apps = append(apps.apps, app)
}
