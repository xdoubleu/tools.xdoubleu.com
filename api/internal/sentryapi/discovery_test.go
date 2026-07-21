package sentryapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/oauthconn"
	"tools.xdoubleu.com/internal/sentryapi"
)

func TestListOrgs_ReturnsOrgs(t *testing.T) {
	var gotPath string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"slug":"org-a"},{"slug":"org-b"}]`))
		}))
	defer cleanup()

	c := sentryapi.New(logging.NewNopLogger(), stubToken("token"), configWith(""))
	orgs, err := c.ListOrgs(context.Background())
	require.NoError(t, err)
	require.Len(t, orgs, 2)
	assert.Equal(t, "org-a", orgs[0].Slug)
	assert.True(t, strings.HasSuffix(gotPath, "/api/0/organizations/"))
}

func TestListProjects_ReturnsProjects(t *testing.T) {
	var gotPath string
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"slug":"proj-a"}]`))
		}))
	defer cleanup()

	c := sentryapi.New(logging.NewNopLogger(), stubToken("token"), configWith(""))
	projects, err := c.ListProjects(context.Background(), "org-a")
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, "proj-a", projects[0].Slug)
	assert.True(t, strings.HasSuffix(gotPath, "/api/0/organizations/org-a/projects/"))
}

func TestListOrgs_NotConnected(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	c := sentryapi.New(logging.NewNopLogger(), stubNotConnected(), configWith(""))
	_, err := c.ListOrgs(context.Background())
	require.ErrorIs(t, err, oauthconn.ErrNotConnected)
	assert.False(t, called, "must not hit the API when not connected")
}
