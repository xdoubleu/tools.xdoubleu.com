package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/sentryapi"
)

// mcpToolCount is the number of read-only observability tools the server exposes.
const mcpToolCount = 7

// bearerRoundTripper attaches a Bearer token to every MCP client request,
// standing in for the OAuth access token a real client would send.
type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if b.token != "" {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return b.base.RoundTrip(r)
}

// mcpSession connects an in-process MCP client to the /monitoring/mcp endpoint
// of a freshly started test server, authenticating with the given Bearer token.
func mcpSession(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	ts := connectServer(t)

	transport := &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/monitoring/mcp",
		HTTPClient: &http.Client{
			Transport: bearerRoundTripper{token: token, base: http.DefaultTransport},
		},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
		OAuthHandler:         nil,
	}

	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(
		&mcp.Implementation{Name: "test", Version: "0"}, nil,
	)
	session, err := client.Connect(context.Background(), transport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestMonitoringMCPProtectedResourceMetadata(t *testing.T) {
	ts := connectServer(t)

	resp, err := ts.Client().Get(
		ts.URL + "/.well-known/oauth-protected-resource/monitoring/mcp",
	)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var md struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&md))
	assert.Equal(t, testApp.mcpResourceURL(), md.Resource)
	require.Len(t, md.AuthorizationServers, 1)
	assert.Equal(t, testApp.mcpAuthServerIssuer(), md.AuthorizationServers[0])
}

func TestMonitoringMCPUnauthenticated(t *testing.T) {
	ts := connectServer(t)

	// No Bearer token: the resource server must challenge with 401 and point at
	// the protected-resource metadata so the client can start the OAuth flow.
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, ts.URL+"/monitoring/mcp", nil,
	)
	require.NoError(t, err)
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t,
		resp.Header.Get("WWW-Authenticate"), "resource_metadata")
}

func TestMonitoringMCPListToolsAsAdmin(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	session := mcpSession(t, accessToken.Value)
	res, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := make([]string, len(res.Tools))
	for i, tool := range res.Tools {
		names[i] = tool.Name
	}
	assert.Len(t, names, mcpToolCount)
	assert.Contains(t, names, "get_job_stats")
	assert.Contains(t, names, "get_github_issues")
	assert.Contains(t, names, "get_deploy_status")
}

func TestMonitoringMCPCallEveryTool(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	// Unconfigured external clients degrade gracefully; the DB-backed tools run
	// against the (empty) test schema. Every tool must return a non-error result.
	testApp.githubClient = github.New(
		logging.NewNopLogger(),
		stubTok(""),
		configNotConnected(),
	)
	testApp.sentryClient = sentryapi.New(
		logging.NewNopLogger(),
		stubTok(""),
		configNotConnected(),
	)
	testApp.doClient = digitalocean.New(
		logging.NewNopLogger(),
		stubTok(""),
		configNotConnected(),
	)

	session := mcpSession(t, accessToken.Value)
	tools := []string{
		"get_job_stats", "get_usage_stats", "get_storage_stats",
		"get_database_stats", "get_github_issues", "get_sentry_issues",
		"get_deploy_status",
	}
	for _, name := range tools {
		//nolint:exhaustruct // only the tool name is required to call it
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: name,
		})
		require.NoErrorf(t, err, "tool %s", name)
		assert.Falsef(t, res.IsError, "tool %s returned an error result", name)
		require.Lenf(t, res.Content, 1, "tool %s", name)
		_, ok := res.Content[0].(*mcp.TextContent)
		assert.Truef(t, ok, "tool %s did not return text content", name)
	}
}

func TestMonitoringMCPInvalidToken(t *testing.T) {
	ts := connectServer(t)

	// A well-formed but unresolvable Bearer token exercises the verifier's
	// error path and must yield a 401 challenge.
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, ts.URL+"/monitoring/mcp", nil,
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestMonitoringMCPCallToolReturnsData(t *testing.T) {
	promoteToAdmin(t)
	t.Cleanup(func() { demoteToUser(t) })

	srv := jsonServer(t, http.StatusOK, `[
		{"number":7,"title":"MCPBug","html_url":"u","state":"open",
		 "created_at":"2026-07-01T00:00:00Z","labels":[{"name":"bug"}]}
	]`)
	github.SetBaseURL(srv.URL)
	t.Cleanup(func() { github.SetBaseURL("https://api.github.com") })
	testApp.githubClient = github.New(
		logging.NewNopLogger(), stubTok("tok"),
		testConfigJSON(t, map[string]string{"repo": "o/r"}),
	)

	session := mcpSession(t, accessToken.Value)
	//nolint:exhaustruct // only the tool name is required to call it
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_github_issues",
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	require.Len(t, res.Content, 1)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "MCPBug")
	assert.Contains(t, text.Text, `"configured":true`)
}

func TestMonitoringMCPCallToolNonAdmin(t *testing.T) {
	demoteToUser(t)

	session := mcpSession(t, accessToken.Value)
	//nolint:exhaustruct // only the tool name is required to call it
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_job_stats",
	})

	// A handler error surfaces as an error result the model can see, not a
	// protocol-level failure; accept either shape but require the denial.
	if err != nil {
		assert.Contains(t, strings.ToLower(err.Error()), "admin")
		return
	}
	require.True(t, res.IsError)
	require.Len(t, res.Content, 1)
	text, ok := res.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, strings.ToLower(text.Text), "admin")
}
