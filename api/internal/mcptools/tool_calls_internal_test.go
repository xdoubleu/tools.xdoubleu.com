package mcptools

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type toolHandler = mcp.ToolHandlerFor[NoArgs, any]

func instant(context.Context, *mcp.CallToolRequest, NoArgs) (
	*mcp.CallToolResult, any, error,
) {
	//nolint:exhaustruct // an empty result
	return &mcp.CallToolResult{}, nil, nil
}

// toolServer is a server with one tool, "the_tool", behind boundToolCalls.
func toolServer(
	logs *bytes.Buffer, threshold, slack time.Duration, tool toolHandler,
) *mcp.Server {
	logger := slog.New(slog.NewTextHandler(logs, nil))
	//nolint:exhaustruct // only Name/Version identify the test server
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	srv.AddReceivingMiddleware(boundToolCalls(logger, threshold, slack))
	//nolint:exhaustruct // the name is all the tool needs
	mcp.AddTool(srv, &mcp.Tool{Name: "the_tool"}, tool)
	return srv
}

func callTool(
	t *testing.T, session *mcp.ClientSession,
) *mcp.CallToolResult {
	t.Helper()
	//nolint:exhaustruct // the name is all the tool needs
	res, err := session.CallTool(
		context.Background(), &mcp.CallToolParams{Name: "the_tool"},
	)
	require.NoError(t, err)
	return res
}

func inMemorySession(t *testing.T, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err := srv.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	return clientSession(t, clientTransport)
}

func clientSession(t *testing.T, transport mcp.Transport) *mcp.ClientSession {
	t.Helper()
	//nolint:exhaustruct // only Name/Version identify the test client
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(context.Background(), transport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// Every method passes through the middleware, but only the tool call is
// logged, with its name.
func TestBoundToolCallsLogsSlowCall(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	srv := toolServer(&logs, time.Nanosecond, time.Second, instant)

	callTool(t, inMemorySession(t, srv))

	assert.Equal(t, 1, strings.Count(logs.String(), "slow MCP tool call"))
	assert.Contains(t, logs.String(), "level=WARN")
	assert.Contains(t, logs.String(), "tool=the_tool")
	assert.Contains(t, logs.String(), "duration=")
	assert.Contains(t, logs.String(), "expired=false")
}

func TestBoundToolCallsQuietBelowThreshold(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	srv := toolServer(&logs, time.Hour, time.Second, instant)

	callTool(t, inMemorySession(t, srv))

	assert.Empty(t, logs.String())
}

// The SDK runs a tool call under a context without the HTTP request's
// deadline. Restored, less the slack, a stuck tool's error still reaches the
// client before the request's deadline ends the response.
func TestBoundToolCallsKeepsRequestDeadline(t *testing.T) {
	t.Parallel()
	const (
		budget = 1500 * time.Millisecond
		slack  = time.Second
	)
	var logs bytes.Buffer
	srv := toolServer(&logs, time.Nanosecond, slack, func(
		ctx context.Context, _ *mcp.CallToolRequest, _ NoArgs,
	) (*mcp.CallToolResult, any, error) {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(budget):
			return nil, nil, errors.New("no deadline")
		}
	})
	//nolint:exhaustruct // a stateless server, as in production
	opts := &mcp.StreamableHTTPOptions{Stateless: true}
	handler := KeepRequestDeadline(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, opts,
	))
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), budget)
			defer cancel()
			handler.ServeHTTP(w, r.WithContext(ctx))
		},
	))
	t.Cleanup(ts.Close)

	res := callTool(t, clientSession(t, &mcp.StreamableClientTransport{
		Endpoint:             ts.URL,
		HTTPClient:           ts.Client(),
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
		OAuthHandler:         nil,
		MaxEventSize:         0,
	}))

	assert.True(t, res.IsError)
	assert.Contains(t, logs.String(), "tool=the_tool")
	assert.Contains(t, logs.String(), "expired=true")
}
