package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/dashboard"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/shoppinglist"
	"tools.xdoubleu.com/apps/todos"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/notifications"
	"tools.xdoubleu.com/internal/repositories"
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
	notifications *notifications.Service,
	appUsersRepo *repositories.AppUsersRepository,
) (*Apps, *books.Books, *feeds.Feeds) {
	var apps Apps = []App{}

	// Migrations run sequentially in registration order: books must adopt its
	// tables from the former backlog schema before games' final migration
	// drops that schema, so books registers before games (this also matches
	// the alphabetical package order used by `go test -p 1 ./...`). feeds
	// registers immediately after books because its own migration copies
	// feed/item data out of books.feeds/books.feed_items/books.books/
	// books.user_books before dropping the reading-era tables (issue #734)
	// — those tables must still exist when feeds' migration runs. dashboard
	// registers last (it has no migrations of its own) and is constructed
	// with live references to books/feeds/games, since its public RPCs
	// delegate to their exported methods instead of duplicating any
	// business logic or querying their schemas directly (issue #737).
	booksApp := books.New(authService, logger, cfg, db)
	apps.addApp(booksApp)
	feedsApp := feeds.New(authService, logger, cfg, db, notifications, appUsersRepo)
	apps.addApp(feedsApp)
	gamesApp := games.New(authService, logger, cfg, db)
	apps.addApp(gamesApp)
	apps.addApp(watchparty.New(authService, logger, cfg))
	apps.addApp(recipes.New(authService, logger, cfg, db))
	apps.addApp(mealplans.New(authService, logger, cfg, db))
	apps.addApp(shoppinglist.New(authService, logger, cfg, db))
	apps.addApp(todos.New(authService, logger, cfg, db))
	apps.addApp(
		dashboard.New(authService, logger, cfg, db, gamesApp, booksApp, feedsApp),
	)

	return &apps, booksApp, feedsApp
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
