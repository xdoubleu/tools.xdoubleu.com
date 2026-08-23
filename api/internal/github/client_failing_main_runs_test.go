package github_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
)

func TestListFailingMainRuns_ReturnsOnlyNonPassingRuns(t *testing.T) {
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workflow_runs":[
				{"id":1,"name":"CI","html_url":"https://gh/runs/1",
				 "conclusion":"failure","head_sha":"sha1",
				 "updated_at":"2026-07-01T10:00:00Z"},
				{"id":2,"name":"CI","html_url":"https://gh/runs/2",
				 "conclusion":"success","head_sha":"sha2",
				 "updated_at":"2026-07-02T10:00:00Z"}
			]}`))
		}))
	defer cleanup()

	runs, err := newClient().ListFailingMainRuns(context.Background())
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, int64(1), runs[0].ID)
	assert.Equal(t, "CI", runs[0].WorkflowName)
	assert.Equal(t, "failure", runs[0].Conclusion)
	assert.Equal(t, "sha1", runs[0].HeadSHA)
}

func TestListFailingMainRuns_NotConfigured_NoConnection(t *testing.T) {
	c := github.New(logging.NewNopLogger(), stubToken("unused"), configNotConnected())
	_, err := c.ListFailingMainRuns(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestListFailingMainRuns_NotConfigured_NotConnected(t *testing.T) {
	c := github.New(
		logging.NewNopLogger(),
		stubNotConnected(),
		configWithRepo(testRepo),
	)
	_, err := c.ListFailingMainRuns(context.Background())
	require.ErrorIs(t, err, github.ErrNotConfigured)
}

func TestListFailingMainRuns_CachesResult(t *testing.T) {
	requests := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workflow_runs":[]}`))
		}))
	defer cleanup()

	c := newClient()
	_, err := c.ListFailingMainRuns(context.Background())
	require.NoError(t, err)
	_, err = c.ListFailingMainRuns(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, requests, "second call must be served from cache")
}

func TestListFailingMainRuns_ServerError_Retries(t *testing.T) {
	attempts := 0
	cleanup := buildServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
	defer cleanup()

	_, err := newClient().ListFailingMainRuns(context.Background())
	require.Error(t, err)
	assert.Equal(t, 4, attempts, "5xx must retry up to maxAttempts")
}
