package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
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
	for name := range disabled {
		mux.Handle("/"+name+"/", disabledAppHandler(name))
	}

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
