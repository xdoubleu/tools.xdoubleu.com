package main

import (
	"log/slog"
	"net/http"

	"github.com/XDoubleU/essentia/pkg/database/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	goaltracker "tools.xdoubleu.com/apps/goaltracker"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
)

type Apps struct {
	apps []App
}

type App interface {
	Routes(prefix string, mux *http.ServeMux) http.Handler
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
