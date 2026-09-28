package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
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

func TestListRepositoryVariables_ReturnsVariables(t *testing.T) {
	body := `{"total_count":2,"variables":[
		{"name":"ROUTINE_NIGHTLY_MAINTENANCE_SWEEP_ENABLED","value":"true"},
		{"name":"A_OTHER","value":"x"}]}`
	cleanup := buildServer(jsonHandler(http.StatusOK, body))
	defer cleanup()

	c := github.New(logging.NewNopLogger(), stubToken("token"), configWithRepo(testRepo))
	vars, err := c.ListRepositoryVariables(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"ROUTINE_NIGHTLY_MAINTENANCE_SWEEP_ENABLED": "true",
		"A_OTHER": "x",
	}, vars)
}

func TestListRepositoryVariables_Paginates(t *testing.T) {
	pages := []string{
		`{"total_count":2,"variables":[{"name":"v1","value":"a"}]}`,
		`{"total_count":2,"variables":[{"name":"v2","value":"b"}]}`,
	}
	call := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			if call >= len(pages) {
				t.Fatalf("unexpected page %d", call)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(pages[call]))
			call++
		}))
	defer cleanup()

	c := github.New(logging.NewNopLogger(), stubToken("token"), configWithRepo(testRepo))
	vars, err := c.ListRepositoryVariables(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"v1": "a", "v2": "b"}, vars)
	assert.Equal(t, 2, call, "must fetch until total_count is reached")
}

func TestListRepositoryVariables_NotConnected(t *testing.T) {
	called := false
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
	defer cleanup()

	c := github.New(logging.NewNopLogger(), stubNotConnected(), configWithRepo(testRepo))
	_, err := c.ListRepositoryVariables(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
	assert.False(t, called, "must not hit the API when not connected")
}
