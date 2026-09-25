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
	"tools.xdoubleu.com/apps/learningpaths"
	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/shoppinglist"
	"tools.xdoubleu.com/apps/trains"
	"tools.xdoubleu.com/apps/watchparty"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/crypto"
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

// MCPToolProvider is optionally implemented by apps exposing tools on
// /apps/mcp.
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
	familyRepo *repositories.FamilyRepository,
	authSealer *crypto.Sealer,
) (*Apps, *books.Books, *feeds.Feeds) {
	var apps Apps = []App{}

	// Migrations run in registration order. books adopts tables from the backlog
	// schema before games' migration drops it; feeds copies data out of books'
	// reading-era tables before they're dropped, so it follows books. dashboard
	// has no migrations and delegates to the other apps' exported methods.
	booksApp := books.New(authService, logger, cfg, db)
	apps.addApp(booksApp)
	feedsApp := feeds.New(authService, logger, cfg, db, notifications, appUsersRepo)
	apps.addApp(feedsApp)
	gamesApp := games.New(authService, logger, cfg, db)
	apps.addApp(gamesApp)
	apps.addApp(watchparty.New(authService, logger, cfg))
	apps.addApp(recipes.New(authService, logger, cfg, db, familyRepo))
	apps.addApp(mealplans.New(authService, logger, cfg, db, familyRepo))
	apps.addApp(shoppinglist.New(authService, logger, cfg, db, familyRepo))
	apps.addApp(
		dashboard.New(authService, logger, cfg, db, gamesApp, booksApp, feedsApp),
	)
	// trains has no schema dependencies, so its position is free.
	apps.addApp(trains.New(authService, logger, cfg, db))
	// learningpaths has no schema dependencies. It takes booksApp/feedsApp to
	// resolve linked resources, and authSealer for its own per-user Todoist
	// OAuth connections.
	apps.addApp(
		learningpaths.New(
			authService, logger, cfg, db, authSealer, booksApp, feedsApp,
		),
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
