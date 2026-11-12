package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// appDependencies lists, per app, the apps it needs to be healthy: schema order
// and exported-method imports. An app is disabled when any of them is.
var appDependencies = map[string][]string{
	"dashboard":     {"books", "feeds", "games"},
	"learningpaths": {"books", "feeds"},
	"feeds":         {"books"},
	"games":         {"books"}, // adopts tables from books' backlog schema
	"mealplans":     {"recipes"},
	"shoppinglist":  {"mealplans", "recipes"},
	"taste":         {"books", "feeds", "games", "movies", "podcasts"},
}

var appMigrationFailed = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "app_migration_failed",
		Help: "1 while an app is disabled because its migration (or a dependency's) failed.",
	},
	[]string{"app"},
)

// AppFailure is an app that was disabled at boot.
type AppFailure struct {
	App string
	Err error
}

// disabledApp stands in for an app whose migration failed: it answers 503 on
// its routes, starts nothing and registers no MCP tools or workers.
type disabledApp struct {
	App
	scratch *http.ServeMux
}

// Routes records the app's routes on a private mux so disabledAppsMiddleware
// can recognise them; nothing is mounted.
func (d *disabledApp) Routes(prefix string, _ *http.ServeMux) {
	d.scratch = http.NewServeMux()
	d.App.Routes(prefix, d.scratch)
}

func (d *disabledApp) ApplyMigrations(context.Context, *pgxpool.Pool) error { return nil }
func (d *disabledApp) Start() error                                         { return nil }
func (d *disabledApp) Shutdown(context.Context) error                       { return nil }

func (d *disabledApp) owns(r *http.Request) bool {
	if d.scratch == nil {
		return false
	}
	_, pattern := d.scratch.Handler(r)
	return pattern != "" && !strings.HasSuffix(pattern, " /") && pattern != "/"
}

// disabledAppsMiddleware answers 503 for requests to a disabled app's routes.
func (app *Application) disabledAppsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, a := range *app.apps {
			if d, ok := a.(*disabledApp); ok && d.owns(r) {
				http.Error(w, "this app is temporarily unavailable",
					http.StatusServiceUnavailable)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ApplyMigrations migrates each app in registration order. A failing app, or
// one whose dependency is disabled, is replaced by a disabledApp and reported
// in the result; the others carry on.
func (apps *Apps) ApplyMigrations(
	ctx context.Context,
	db *pgxpool.Pool,
	logger *slog.Logger,
) []AppFailure {
	var failures []AppFailure
	disabled := map[string]bool{}

	for i, a := range *apps {
		name := a.GetName()

		var err error
		if dep := slices.IndexFunc(appDependencies[name], func(d string) bool {
			return disabled[d]
		}); dep >= 0 {
			err = fmt.Errorf("dependency %s is disabled", appDependencies[name][dep])
		} else if migrateErr := a.ApplyMigrations(ctx, db); migrateErr != nil {
			err = migrateErr
			logger.Error("app migration failed; disabling app",
				"app", name, "error", err)
			sentry.CaptureException(fmt.Errorf("migrate %s: %w", name, err))
		}
		if err == nil {
			continue
		}

		disabled[name] = true
		appMigrationFailed.WithLabelValues(name).Set(1)
		failures = append(failures, AppFailure{App: name, Err: err})
		(*apps)[i] = &disabledApp{App: a, scratch: nil}
	}

	return failures
}
