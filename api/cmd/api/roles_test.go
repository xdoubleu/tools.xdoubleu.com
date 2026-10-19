package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/logging"
)

type roleApp struct {
	App
	started, workersStarted int
}

func (a *roleApp) Start() error { a.started++; return nil }

type workersRoleApp struct{ roleApp }

//nolint:unparam //satisfies WorkersStarter
func (a *workersRoleApp) StartWorkers() error { a.workersStarted++; return nil }

func TestAppsStartByRole(t *testing.T) {
	plain := &roleApp{App: nil, started: 0, workersStarted: 0}
	heavy := &workersRoleApp{roleApp{App: nil, started: 0, workersStarted: 0}}
	apps := Apps{plain, heavy}

	require.NoError(t, apps.Start(config.RoleWorkers))
	assert.Zero(t, plain.started)
	assert.Zero(t, heavy.started)
	assert.Equal(t, 1, heavy.workersStarted)

	require.NoError(t, apps.Start(config.RoleWeb))
	assert.Equal(t, 1, plain.started)
	assert.Equal(t, 1, heavy.started)
	assert.Equal(t, 1, heavy.workersStarted)
}

func TestHeapWatchThresholdBelowRoleCaps(t *testing.T) {
	const mib = 1 << 20
	assert.Less(t, heapWatchThreshold(config.RoleWeb), uint64(512*mib))
	assert.Less(t, heapWatchThreshold(config.RoleWorkers), uint64(1536*mib))
}

func TestWorkersHandlerServesOnlyHealthAndMetrics(t *testing.T) {
	//nolint:exhaustruct //only config is read
	app := &Application{config: config.Config{Role: config.RoleWorkers}}
	h := app.handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, metricsPath, nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/version", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewApplicationWorkersRoleServesHealth(t *testing.T) {
	cfg := testApp.config
	cfg.Role = config.RoleWorkers
	workers := NewApplication(logging.NewNopLogger(), cfg, testApp.db)

	rec := httptest.NewRecorder()
	workers.handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, healthPath, nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}
