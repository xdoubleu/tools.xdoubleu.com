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

func TestObservabilityGetProjectIssuesByStatus_AsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"user":{"projectV2":{"items":{"nodes":[
				{"status":{"name":"Ready"},
				 "content":{"number":1357,"title":"Add MCP tool",
				            "url":"https://gh/issues/1357","state":"OPEN"}}
			]}}}}}`))
		}))
	t.Cleanup(srv.Close)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	resp, err := callProjectIssuesByStatus(t, 8, "Ready")
	require.NoError(t, err)
	assert.True(t, resp.Msg.Configured)
	require.Len(t, resp.Msg.Issues, 1)
	assert.Equal(t, int64(1357), resp.Msg.Issues[0].Number)
	assert.Equal(t, "Ready", resp.Msg.Issues[0].Status)
}

func TestObservabilityGetProjectIssuesByStatus_NotConfigured(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok("tok"),
		configNotConnected(),
	)

	resp, err := callProjectIssuesByStatus(t, 8, "Ready")
	require.NoError(t, err)
	assert.False(t, resp.Msg.Configured)
	assert.Empty(t, resp.Msg.Issues)
}

func TestObservabilityGetProjectIssuesByStatus_NonAdmin(t *testing.T) {
	demoteToUser(t)
	_, err := callProjectIssuesByStatus(t, 8, "Ready")
	requirePermissionDenied(t, err)
}

func callProjectIssuesByStatus(
	t *testing.T, projectNumber int32, status string,
) (*connect.Response[observabilityv1.GetProjectIssuesByStatusResponse], error) {
	t.Helper()
	req := connect.NewRequest(&observabilityv1.GetProjectIssuesByStatusRequest{
		ProjectNumber: projectNumber,
		Status:        status,
	})
	setCookieOnRequest(req, accessToken)
	return observabilityClient(t).GetProjectIssuesByStatus(context.Background(), req)
}
