package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

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
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/crypto"
	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/account"
	"tools.xdoubleu.com/internal/platform/agentaccounts"
	"tools.xdoubleu.com/internal/platform/auth"
	"tools.xdoubleu.com/internal/platform/family"
	"tools.xdoubleu.com/internal/platform/githubapp"
	"tools.xdoubleu.com/internal/platform/notifications"
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
	appUsersRepo *account.AppUsersRepository,
	familyRepo *family.Repository,
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
	mealplansApp := mealplans.New(ctx, authService, logger, cfg, db, familyRepo)
	apps.addApp(mealplansApp)
	shoppinglistApp := shoppinglist.New(ctx, authService, logger, cfg, db, familyRepo)
	apps.addApp(shoppinglistApp)
	// trains has no schema dependencies, so its position is free.
	apps.addApp(trains.New(ctx, authService, logger, cfg, db))
	// learningpaths has no schema dependencies. It takes booksApp/feedsApp to
	// resolve linked resources, and authSealer for its own per-user Todoist
	// OAuth connections.
	learningpathsApp := learningpaths.New(
		ctx,
		authService, logger, cfg, db, authSealer, booksApp, feedsApp,
	)
	apps.addApp(learningpathsApp)
	// movies has no schema dependencies.
	moviesApp := movies.New(ctx, authService, logger, cfg, db, familyRepo)
	apps.addApp(moviesApp)

	// podcasts has no schema dependencies.
	podcastsApp := podcasts.New(ctx, authService, logger, cfg, db)
	apps.addApp(podcastsApp)

	// factory has no schema dependencies and imports no other app.
	var factoryOpts []factory.Option
	if stepUp, ok := authService.(factory.StepUp); ok {
		factoryOpts = append(factoryOpts, factory.WithStepUp(stepUp))
	}
	if invalidator, ok := authService.(interface{ InvalidateUserCache() }); ok {
		jobAccounts := agentaccounts.NewStore(db, invalidator.InvalidateUserCache)
		factoryOpts = append(factoryOpts, factory.WithJobAccounts(jobAccounts))
	}
	factoryOpts = append(factoryOpts, factoryGitHubOptions(cfg, logger)...)
	factoryOpts = append(factoryOpts,
		factory.WithGitHubWebhookSecret(cfg.FactoryGitHubWebhookSecret),
		factory.WithSlack(cfg.FactorySlackWebhookURL, cfg.WebURL))
	factoryApp := factory.New(ctx, authService, logger, cfg, db, factoryOpts...)
	apps.addApp(factoryApp)

	// dashboard has no migrations and summarises the apps above on its Today
	// page, so it follows them.
	apps.addApp(dashboard.New(
		ctx, authService, logger, cfg, db, gamesApp, booksApp, feedsApp,
		dashboard.TodayApps{
			Books:         booksApp,
			Feeds:         feedsApp,
			MealPlans:     mealplansApp,
			ShoppingList:  shoppinglistApp,
			Movies:        moviesApp,
			LearningPaths: learningpathsApp,
			Factory:       factoryApp,
		},
	))

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

const (
	appBooks = "books"
	appFeeds = "feeds"
)

// appDependsOn lists the apps whose schema an app's migration relies on; an
// app is skipped when one of them failed. Registration order already respects
// it.
//
//nolint:gochecknoglobals //static registration data
var appDependsOn = map[string][]string{
	appFeeds:        {appBooks},
	"games":         {appBooks},
	"dashboard":     {appBooks, appFeeds, "games"},
	"learningpaths": {appBooks, appFeeds},
	"mealplans":     {"recipes"},
	"shoppinglist":  {"mealplans", "recipes"},
}

// DisabledApps maps an app's name to why it is disabled.
type DisabledApps map[string]error

// ApplyMigrations migrates each app in order. A failing app, or one whose
// dependency failed, is returned as disabled instead of aborting the rest.
func (apps *Apps) ApplyMigrations(
	ctx context.Context,
	db *pgxpool.Pool,
) DisabledApps {
	disabled := DisabledApps{}
	for _, app := range *apps {
		name := app.GetName()
		skipped := false
		for _, dep := range appDependsOn[name] {
			if _, failed := disabled[dep]; failed {
				disabled[name] = fmt.Errorf("dependency %s is disabled", dep)
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}
		if err := app.ApplyMigrations(ctx, db); err != nil {
			disabled[name] = err
		}
	}
	return disabled
}

// Without returns the apps not in disabled.
func (apps *Apps) Without(disabled DisabledApps) *Apps {
	if len(disabled) == 0 {
		return apps
	}
	enabled := make(Apps, 0, len(*apps))
	for _, app := range *apps {
		if _, off := disabled[app.GetName()]; !off {
			enabled = append(enabled, app)
		}
	}
	return &enabled
}

// Names returns the disabled app names, sorted.
func (d DisabledApps) Names() []string {
	names := make([]string, 0, len(d))
	for name := range d {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (apps *Apps) Routes(mux *http.ServeMux) {
	for _, app := range *apps {
		app.Routes(app.GetName(), mux)
	}
}

func (apps *Apps) addApp(app App) {
	*apps = append(*apps, app)
}

// factoryGitHubOptions lets jobs push when the factory GitHub App is set up. Unset,
// there is nothing to do; half set, or a key that doesn't parse, it says so and
// leaves pushing off rather than refusing to start.
func factoryGitHubOptions(cfg config.Config, logger *slog.Logger) []factory.Option {
	gh, err := githubapp.New(cfg.FactoryGitHubAppID, cfg.FactoryGitHubAppKey)
	if err == nil {
		return []factory.Option{factory.WithGitHub(gh)}
	}
	if cfg.FactoryGitHubAppID != "" || cfg.FactoryGitHubAppKey != "" {
		logger.Warn("factory GitHub App is not usable; jobs will not push", "error", err)
	}
	return nil
}
