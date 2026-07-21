package digitalocean_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/oauthconn"
)

func TestListApps_ReturnsApps(t *testing.T) {
	body := `{"apps":[
		{"id":"id-1","spec":{"name":"app-one"}},
		{"id":"id-2","spec":{"name":"app-two"}}
	]}`
	cleanup := buildServer(jsonHandler(http.StatusOK, body))
	defer cleanup()

	c := digitalocean.New(
		logging.NewNopLogger(),
		stubToken("token"),
		configWithAppID(""),
	)
	apps, err := c.ListApps(context.Background())
	require.NoError(t, err)
	require.Len(t, apps, 2)
	assert.Equal(t, "id-1", apps[0].ID)
	assert.Equal(t, "app-one", apps[0].Name)
	assert.Equal(t, "id-1 — app-one", apps[0].Option())
}

func TestListApps_NotConnected(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	c := digitalocean.New(
		logging.NewNopLogger(),
		stubNotConnected(),
		configWithAppID(""),
	)
	_, err := c.ListApps(context.Background())
	require.ErrorIs(t, err, oauthconn.ErrNotConnected)
	assert.False(t, called, "must not hit the API when not connected")
}

func TestAppIDFromOption(t *testing.T) {
	assert.Equal(t, "id-1", digitalocean.AppIDFromOption("id-1 — app-one"))
	assert.Equal(
		t,
		"id-1",
		digitalocean.AppIDFromOption("id-1"),
		"already-bare IDs pass through",
	)
}

func jsonHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}
