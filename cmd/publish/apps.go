package main

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
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
	Start() error
}

func NewApps(
	ctx context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sharedTpl *template.Template,
) *Apps {
	apps := &Apps{
		apps: []App{},
	}

	apps.addApp(goaltracker.New(ctx, authService, logger, cfg, db, sharedTpl))
	apps.addApp(watchparty.New(authService, logger, cfg, sharedTpl))
	apps.addApp(icsproxy.New(authService, logger, cfg, db, sharedTpl))

	return apps
}

func (apps *Apps) ApplyMigrations(db *pgxpool.Pool) error {
	for _, app := range apps.apps {
		goose.SetTableName(fmt.Sprintf("%s.goose_db_version", app.GetName()))
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
