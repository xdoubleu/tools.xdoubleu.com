package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/testhelper"
)

func TestNewSupabaseClientCustomAuthURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(map[string]string{"name": "GoTrue"})
			require.NoError(t, err)
		},
	))
	defer srv.Close()

	cfg := testhelper.NewTestConfig()
	cfg.SupabaseProjRef = "proj-ref"
	cfg.SupabaseAPIKey = "api-key"
	cfg.GoTrueURL = srv.URL

	client := newSupabaseClient(cfg)

	health, err := client.HealthCheck()
	require.NoError(t, err)
	assert.Equal(t, "GoTrue", health.Name)
}
