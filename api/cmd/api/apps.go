package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/icsproxy"
	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/reading"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/shoppinglist"
	"tools.xdoubleu.com/apps/todos"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
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

// MCPToolProvider is implemented by apps that expose read-only MCP tools on the
// combined /apps/mcp server. It is optional: newAppsMCPServer only registers
// tools for apps that satisfy it (see mcp_apps.go).
type MCPToolProvider interface {
	RegisterMCPTools(srv *mcp.Server)
}

func NewApps(
	authService auth.Service,
	logger *slog.Logger,
	cfg config.Config,
	db postgres.DB,
) (*Apps, *reading.Reading) {
	var apps Apps = []App{}

	// Migrations run sequentially in registration order: books must adopt its
	// tables from the former backlog schema before games' final migration
	// drops that schema, so books registers before games (this also matches
	// the alphabetical package order used by `go test -p 1 ./...`).
	readingApp := reading.New(authService, logger, cfg, db)
	apps.addApp(readingApp)
	apps.addApp(games.New(authService, logger, cfg, db))
	apps.addApp(watchparty.New(authService, logger, cfg))
	apps.addApp(icsproxy.New(authService, logger, cfg, db))
	apps.addApp(recipes.New(authService, logger, cfg, db))
	apps.addApp(mealplans.New(authService, logger, cfg, db))
	apps.addApp(shoppinglist.New(authService, logger, cfg, db))
	apps.addApp(todos.New(authService, logger, cfg, db))

	return &apps, readingApp
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
