package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/test"

	"tools.xdoubleu.com/internal/testhelper"
)

func TestHealthEndpointSuccess(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/health",
	)

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)

	var response map[string]string
	require.NoError(t, json.NewDecoder(rs.Body).Decode(&response))
	assert.Equal(t, "ok", response["status"])
}

func TestHealthEndpointNoAuth(t *testing.T) {
	tReq := test.CreateRequestTester(
		testApp.Routes(),
		http.MethodGet,
		"/health",
	)
	// Intentionally not adding auth cookie

	rs := tReq.Do(t)
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}

func TestHealthEndpointDBDown(t *testing.T) {
	cfg := testhelper.NewTestConfig()

	pool, err := pgxpool.New(context.Background(), cfg.DBDsn)
	require.NoError(t, err)
	pool.Close()

	//nolint:exhaustruct //only db is used by the handler
	app := &Application{db: pool}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	app.healthHandler(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}
