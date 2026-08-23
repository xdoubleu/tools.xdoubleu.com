package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	observabilityv1 "tools.xdoubleu.com/gen/observability/v1"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
)

func TestObservabilityGetFailingMainRuns_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"workflow_runs":[
				{"id":7,"name":"CI","html_url":"u",
				 "conclusion":"failure","head_sha":"sha1",
				 "updated_at":"2026-07-01T00:00:00Z"}
			]}`))
		}))
	t.Cleanup(srv.Close)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	resp, err := callFailingMainRuns(t)
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Runs, 1)
	assert.Equal(t, int64(7), resp.Msg.Runs[0].Id)
	assert.Equal(t, "failure", resp.Msg.Runs[0].Conclusion)
	assert.Equal(t, int32(1), resp.Msg.FailingCount)
}

func TestObservabilityGetFailingMainRuns_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callFailingMainRuns(t)
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Runs)
}

func TestObservabilityGetFailingMainRuns_UpstreamError(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusInternalServerError, ``)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	resp, err := callFailingMainRuns(t)
	require.NoError(t, err) // degraded, never a failed response
	assert.True(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Runs)
}

func TestObservabilityGetFailingMainRuns_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callFailingMainRuns(t)
	requirePermissionDenied(t, err)
}

func callFailingMainRuns(
	t *testing.T,
) (*connect.Response[observabilityv1.GetFailingMainRunsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetFailingMainRunsRequest{})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetFailingMainRuns(context.Background(), req)
}
