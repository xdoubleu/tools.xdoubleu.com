package sentryapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/sentryapi"
)

func TestListMonitors_ParsesPayload(t *testing.T) {
	body := `[
		{"slug":"resync-books","status":"ok","environments":[
			{"status":"ok","lastCheckIn":"2026-07-25T09:00:00Z",
			 "nextCheckIn":"2026-07-26T09:00:00Z"}
		]},
		{"slug":"poll-feeds","status":"error","environments":[]}
	]`

	var gotPath, authHeader string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			authHeader = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
	defer cleanup()

	monitors, err := newClient().ListMonitors(context.Background())
	require.NoError(t, err)
	require.Len(t, monitors, 2)

	assert.Equal(t, "resync-books", monitors[0].Slug)
	assert.Equal(t, "ok", monitors[0].Status)
	assert.False(t, monitors[0].LastCheckIn.IsZero())
	assert.False(t, monitors[0].NextCheckIn.IsZero())

	assert.Equal(t, "poll-feeds", monitors[1].Slug)
	assert.Equal(
		t,
		"error",
		monitors[1].Status,
		"falls back to top-level status with no environments",
	)
	assert.True(t, monitors[1].LastCheckIn.IsZero())

	assert.True(t, strings.HasSuffix(gotPath, "/api/0/organizations/org/monitors/"))
	assert.Equal(t, "Bearer token", authHeader)
}

func TestListMonitors_NotConfigured(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	cases := []sentryapi.Client{
		sentryapi.New(logging.NewNopLogger(), stubToken("token"), configWith("")),
		sentryapi.New(logging.NewNopLogger(), stubNotConnected(), configWith("org")),
		sentryapi.New(logging.NewNopLogger(), stubToken("token"), configNotConnected()),
	}
	for _, c := range cases {
		_, err := c.ListMonitors(context.Background())
		require.ErrorIs(t, err, sentryapi.ErrNotConfigured)
	}
	assert.False(t, called, "must not hit the API when unconfigured")
}

func TestListMonitors_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				[]byte(`[{"slug":"archive","status":"ok","environments":[]}]`),
			)
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListMonitors(context.Background())
	require.NoError(t, err)
	_, err = c.ListMonitors(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, requests, "second call must be served from cache")
}
