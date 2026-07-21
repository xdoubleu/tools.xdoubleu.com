package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/oauthconn"
)

func TestListRepos_ReturnsRepos(t *testing.T) {
	body := `[{"full_name":"o/a"},{"full_name":"o/b"}]`
	cleanup := buildServer(jsonHandler(http.StatusOK, body))
	defer cleanup()

	c := github.New(logging.NewNopLogger(), stubToken("token"), configWithRepo(""))
	repos, err := c.ListRepos(context.Background())
	require.NoError(t, err)
	require.Len(t, repos, 2)
	assert.Equal(t, "o/a", repos[0].FullName)
	assert.Equal(t, "o/b", repos[1].FullName)
}

func TestListRepos_NotConnected(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	c := github.New(logging.NewNopLogger(), stubNotConnected(), configWithRepo(""))
	_, err := c.ListRepos(context.Background())
	require.ErrorIs(t, err, oauthconn.ErrNotConnected)
	assert.False(t, called, "must not hit the API when not connected")
}
