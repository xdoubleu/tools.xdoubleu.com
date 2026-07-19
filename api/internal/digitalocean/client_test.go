package digitalocean_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/digitalocean"
)

const realBaseURL = "https://api.digitalocean.com"

func TestMain(m *testing.M) {
	digitalocean.SetBackoffBase(1 * time.Millisecond)
	os.Exit(m.Run())
}

func buildServer(handler http.Handler) func() {
	srv := httptest.NewServer(handler)
	digitalocean.SetBaseURL(srv.URL)
	return func() {
		srv.Close()
		digitalocean.SetBaseURL(realBaseURL)
	}
}

func newClient() digitalocean.Client {
	return digitalocean.New(logging.NewNopLogger(), "token", "app-123")
}

func TestLatestDeployment_ReturnsNewest(t *testing.T) {
	body := `{"deployments":[
		{"id":"d1","phase":"ACTIVE","cause":"push",
		 "created_at":"2026-07-10T09:00:00Z","updated_at":"2026-07-10T09:05:00Z"},
		{"id":"d0","phase":"SUPERSEDED","cause":"push",
		 "created_at":"2026-07-09T09:00:00Z","updated_at":"2026-07-09T09:05:00Z"}
	]}`

	var gotPath, authHeader string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
	defer cleanup()

	dep, err := newClient().LatestDeployment(context.Background())
	require.NoError(t, err)
	require.NotNil(t, dep)
	assert.Equal(t, "d1", dep.ID)
	assert.Equal(t, "ACTIVE", dep.Phase)
	assert.Equal(t, "push", dep.Cause)
	assert.Equal(t, "/v2/apps/app-123/deployments", gotPath)
	assert.Equal(t, "Bearer token", authHeader)
}

func TestLatestDeployment_NoDeployments(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"deployments":[]}`))
		}))
	defer cleanup()

	dep, err := newClient().LatestDeployment(context.Background())
	require.NoError(t, err)
	assert.Nil(t, dep)
}

func TestLatestDeployment_NotConfigured(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	cases := []digitalocean.Client{
		digitalocean.New(logging.NewNopLogger(), "", "app-123"),
		digitalocean.New(logging.NewNopLogger(), "token", ""),
	}
	for _, c := range cases {
		_, err := c.LatestDeployment(context.Background())
		require.ErrorIs(t, err, digitalocean.ErrNotConfigured)
	}
	assert.False(t, called, "must not hit the API when unconfigured")
}

func TestLatestDeployment_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(
				`{"deployments":[{"id":"d1","phase":"ACTIVE"}]}`,
			))
		}))
	defer cleanup()

	c := newClient()
	_, err := c.LatestDeployment(context.Background())
	require.NoError(t, err)
	_, err = c.LatestDeployment(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, requests, "second call must be served from cache")
}

func TestLatestDeployment_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadGateway)
		}))
	defer cleanup()

	_, err := newClient().LatestDeployment(context.Background())
	require.Error(t, err)
	assert.Equal(t, 4, attempts)
}

func TestLatestDeployment_NonRetryable4xx(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusUnauthorized)
		}))
	defer cleanup()

	_, err := newClient().LatestDeployment(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, attempts)
}

func TestLatestDeployment_NetworkError(t *testing.T) {
	digitalocean.SetBaseURL("http://127.0.0.1:1")
	defer digitalocean.SetBaseURL(realBaseURL)

	_, err := newClient().LatestDeployment(context.Background())
	require.Error(t, err)
}
