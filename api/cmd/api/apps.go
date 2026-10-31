package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/apps/books"
	"tools.xdoubleu.com/apps/dashboard"
	"tools.xdoubleu.com/apps/factory"
	"tools.xdoubleu.com/apps/feeds"
	"tools.xdoubleu.com/apps/games"
	"tools.xdoubleu.com/apps/learningpaths"
	"tools.xdoubleu.com/apps/mealplans"
	"tools.xdoubleu.com/apps/movies"
	"tools.xdoubleu.com/apps/podcasts"
	"tools.xdoubleu.com/apps/recipes"
	"tools.xdoubleu.com/apps/shoppinglist"
	"tools.xdoubleu.com/apps/taste"
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
	// Shutdown drains the app's background tasks and jobs until ctx is done.
	Shutdown(ctx context.Context) error
}

// WorkersStarter is implemented by apps with jobs for the workers role; the
// workers role calls it instead of Start.
type WorkersStarter interface {
	StartWorkers() error
}

// MCPToolProvider is optionally implemented by apps exposing tools on
// /apps/mcp.
type MCPToolProvider interface {
	RegisterMCPTools(srv *mcp.Server)
}

func NewApps(
	ctx context.Context,
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
	booksApp := books.New(ctx, authService, logger, cfg, db)
	apps.addApp(booksApp)
	feedsApp := feeds.New(ctx, authService, logger, cfg, db, notifications, appUsersRepo)
	apps.addApp(feedsApp)
	gamesApp := games.New(ctx, authService, logger, cfg, db)
	apps.addApp(gamesApp)
	apps.addApp(watchparty.New(ctx, authService, logger, cfg))
	apps.addApp(recipes.New(ctx, authService, logger, cfg, db, familyRepo))
	apps.addApp(mealplans.New(ctx, authService, logger, cfg, db, familyRepo))
	apps.addApp(shoppinglist.New(ctx, authService, logger, cfg, db, familyRepo))
	apps.addApp(
		dashboard.New(ctx, authService, logger, cfg, db, gamesApp, booksApp, feedsApp),
	)
	// trains has no schema dependencies, so its position is free.
	apps.addApp(trains.New(ctx, authService, logger, cfg, db))
	// learningpaths has no schema dependencies. It takes booksApp/feedsApp to
	// resolve linked resources, and authSealer for its own per-user Todoist
	// OAuth connections.
	apps.addApp(
		learningpaths.New(
			ctx,
			authService, logger, cfg, db, authSealer, booksApp, feedsApp,
		),
	)
	// movies has no schema dependencies.
	moviesApp := movies.New(ctx, authService, logger, cfg, db, familyRepo)
	apps.addApp(moviesApp)

	// podcasts has no schema dependencies.
	podcastsApp := podcasts.New(ctx, authService, logger, cfg, db)
	apps.addApp(podcastsApp)

	// factory has no schema dependencies and imports no other app.
	apps.addApp(factory.New(ctx, authService, logger, cfg, db))

	// taste reads every media app through its exported ListConsumption, so it
	// follows them.
	apps.addApp(taste.New(
		ctx,
		authService, logger, cfg, db, familyRepo, notifications,
		taste.Catalogs{
			Books:    booksApp.FindBook,
			Movies:   moviesApp.FindTitle,
			Games:    gamesApp.FindGame,
			Podcasts: podcastsApp.FindShow,
			Feeds:    feedsApp.FindSource,

			WantBook:  booksApp.WantBook,
			WantTitle: moviesApp.WantTitle,
		},
		booksApp, feedsApp, gamesApp, moviesApp, podcastsApp,
	))

	return &apps, booksApp, feedsApp
}

// Start runs each app's jobs for the role: every app's Start for web, only
// StartWorkers of apps that have one for workers.
func (apps *Apps) Start(role string) error {
	for _, app := range *apps {
		if role == config.RoleWorkers {
			if w, ok := app.(WorkersStarter); ok {
				if err := w.StartWorkers(); err != nil {
					return err
				}
			}
			continue
		}
		if err := app.Start(); err != nil {
			return err
		}
	}
	return nil
}

// Shutdown drains every app in turn; ctx bounds the whole drain.
func (apps *Apps) Shutdown(ctx context.Context) error {
	errs := make([]error, 0, len(*apps))
	for _, app := range *apps {
		if err := app.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", app.GetName(), err))
		}
	}
	return errors.Join(errs...)
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
