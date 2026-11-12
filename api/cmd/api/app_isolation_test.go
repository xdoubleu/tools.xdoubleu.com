package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApp struct {
	name       string
	migrateErr error
	started    bool
}

func (f *fakeApp) Routes(_ string, mux *http.ServeMux) {
	mux.HandleFunc("POST /"+f.name+".v1.Service/", func(http.ResponseWriter, *http.Request) {})
}

func (f *fakeApp) ApplyMigrations(context.Context, *pgxpool.Pool) error { return f.migrateErr }
func (f *fakeApp) GetName() string                                      { return f.name }
func (f *fakeApp) GetDisplayName() string                               { return f.name }
func (f *fakeApp) GetDomain() string                                    { return "" }
func (f *fakeApp) Start() error                                         { f.started = true; return nil }
func (f *fakeApp) Shutdown(context.Context) error                       { return nil }

func TestApplyMigrations_DisablesFailedAppAndDependents(t *testing.T) {
	books := &fakeApp{name: "books", migrateErr: errors.New("boom")}
	feeds := &fakeApp{name: "feeds"}
	recipes := &fakeApp{name: "recipes"}
	apps := Apps{books, feeds, recipes}

	failures := apps.ApplyMigrations(context.Background(), nil, slog.Default())

	require.Len(t, failures, 2)
	assert.Equal(t, "books", failures[0].App)
	assert.Equal(t, "feeds", failures[1].App)
	assert.ErrorContains(t, failures[1].Err, "books")

	require.NoError(t, apps.Start("web"))
	assert.False(t, books.started)
	assert.False(t, feeds.started)
	assert.IsType(t, &fakeApp{}, apps[2])

	_, isMCP := apps[0].(MCPToolProvider)
	assert.False(t, isMCP)
}

func TestDisabledAppsMiddleware_Answers503(t *testing.T) {
	books := &fakeApp{name: "books", migrateErr: errors.New("boom")}
	recipes := &fakeApp{name: "recipes"}
	apps := Apps{books, recipes}
	apps.ApplyMigrations(context.Background(), nil, slog.Default())
	apps.Routes(http.NewServeMux())

	app := &Application{apps: &apps}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := app.disabledAppsMiddleware(next)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/books.v1.Service/List", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/recipes.v1.Service/List", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}
