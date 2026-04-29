package main

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/backlog"
	"tools.xdoubleu.com/apps/icsproxy"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/todos"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/contacts"
)

type Apps []App

type App interface {
	Routes(prefix string, mux *http.ServeMux)
	ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error
	GetName() string
	GetDisplayName() string
	GetDomain() string
	Start() error
}

func NewApps(
	_ context.Context,
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
	sharedTpl *template.Template,
	bl *backlog.Backlog,
	contactsSvc contacts.Service,
) *Apps {
	var apps Apps = []App{}

	apps.addApp(bl)
	apps.addApp(watchparty.New(authService, logger, cfg, sharedTpl))
	apps.addApp(icsproxy.New(authService, logger, cfg, db, sharedTpl))
	apps.addApp(recipes.New(authService, logger, cfg, db, sharedTpl, contactsSvc))
	apps.addApp(todos.New(authService, logger, cfg, db, sharedTpl))
	// scaffold:app

	return &apps
}

func (apps *Apps) ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error {
	for _, app := range *apps {
		err := app.ApplyMigrations(ctx, db)
		if err != nil {
			return err
		}
	}
	return nil
}

func (apps *Apps) Routes(mux *http.ServeMux) http.Handler {
	for _, app := range *apps {
		app.Routes(app.GetName(), mux)
	}
	return mux
}

func (apps *Apps) addApp(app App) {
	*apps = append(*apps, app)
}
