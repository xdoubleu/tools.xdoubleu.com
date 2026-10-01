package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/logging"
)

func callFailingPullRequestsTool(
	t *testing.T, status int, body string,
) *mcp.CallToolResult {
	t.Helper()
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, status, body)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	session := appsMCPSession(t, accessToken.Value)
	//nolint:exhaustruct // only the tool name is required to call it
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_failing_pull_requests",
	})
	require.NoError(t, err)
	return res
}

func TestAppsMCPFailingPullRequests_EmptyListIsExplicit(t *testing.T) {
	res := callFailingPullRequestsTool(t, http.StatusOK, `[]`)

	require.False(t, res.IsError, toolMessage(res, nil))
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(toolMessage(res, nil)), &got))
	assert.Equal(t, []any{}, got["pullRequests"])
	assert.InDelta(t, 0, got["failingCount"], 0)
	configured, _ := got["configured"].(bool)
	assert.True(t, configured)
}

func TestAppsMCPFailingPullRequests_UpstreamErrorIsToolError(t *testing.T) {
	res := callFailingPullRequestsTool(t, http.StatusInternalServerError, ``)

	assert.True(t, res.IsError, "an upstream failure must not read as an empty list")
}
