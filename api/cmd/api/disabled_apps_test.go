package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type migrationApp struct {
	App
	name string
	err  error
}

func (a *migrationApp) GetName() string { return a.name }

func (a *migrationApp) ApplyMigrations(context.Context, *pgxpool.Pool) error {
	return a.err
}

func (a *migrationApp) Routes(prefix string, mux *http.ServeMux) {
	mux.HandleFunc("/"+prefix+"/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestApplyMigrationsDisablesFailedAppAndDependents(t *testing.T) {
	apps := Apps{
		&migrationApp{App: nil, name: "books", err: errors.New("boom")},
		&migrationApp{App: nil, name: "feeds", err: nil},
		&migrationApp{App: nil, name: "trains", err: nil},
	}

	disabled := apps.ApplyMigrations(context.Background(), nil)

	assert.Equal(t, []string{"books", "feeds"}, disabled.Names())
	require.ErrorContains(t, disabled["feeds"], "dependency books")

	mux := http.NewServeMux()
	apps.Without(disabled).Routes(mux)
	disabled.Mount(mux)

	for path, want := range map[string]int{
		"/books/x":  http.StatusServiceUnavailable,
		"/feeds/x":  http.StatusServiceUnavailable,
		"/trains/x": http.StatusOK,
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, want, rec.Code, path)
	}
}

func TestHealthBodyDegraded(t *testing.T) {
	assert.Equal(t, "ok", healthBody(nil)["status"])

	body := healthBody(DisabledApps{"books": errors.New("boom")})
	assert.Equal(t, "degraded", body["status"])
	assert.Equal(t, []string{"books"}, body["disabled_apps"])
}

func TestReportDisabledApps(t *testing.T) {
	apps := Apps{&migrationApp{App: nil, name: "books", err: nil}}

	reportDisabledApps(
		slog.New(slog.DiscardHandler),
		apps,
		DisabledApps{"books": errors.New("boom")},
	)

	assert.InDelta(
		t, 1, testutil.ToFloat64(appMigrationFailed.WithLabelValues("books")), 0,
	)
}

func TestPlatformMigrationFailureIsFatal(t *testing.T) {
	ok := Platform{&migrationApp{App: nil, name: "factory", err: nil}}
	require.NoError(t, ok.ApplyMigrations(context.Background(), nil))

	bad := Platform{&migrationApp{App: nil, name: "factory", err: errors.New("boom")}}
	require.ErrorContains(
		t, bad.ApplyMigrations(context.Background(), nil), "factory: boom",
	)
}

func TestPlatformRoutesAndServices(t *testing.T) {
	platform := Platform{&migrationApp{App: nil, name: "factory", err: nil}}
	mux := http.NewServeMux()
	platform.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/factory/x", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	apps := Apps{&migrationApp{App: nil, name: "books", err: nil}}
	assert.Len(t, joinServices(platform, apps), 2)
}
