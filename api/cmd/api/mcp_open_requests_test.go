package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	obs "tools.xdoubleu.com/internal/obs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

// answerWithin is far below the 25s budget a held /apps/mcp POST runs into.
const answerWithin = 2 * time.Second

// sep2575Meta is the per-request _meta a 2026-07-28 client sends.
const sep2575Meta = `"_meta": {
	"io.modelcontextprotocol/protocolVersion": "2026-07-28",
	"io.modelcontextprotocol/clientCapabilities": {}}`

// inFlight counts the requests next is still serving.
type inFlight struct {
	n    atomic.Int32
	next http.Handler
}

func (f *inFlight) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.n.Add(1)
	defer f.n.Add(-1)
	f.next.ServeHTTP(w, r)
}

func noopListChanged[R any](context.Context, R) {}

// A 2026-07-28 client opens a subscriptions/listen POST for each list_changed
// the server advertises. No list ever changes, so once the client's calls are
// answered no POST may stay open: the request budget would cut it at 25s and
// the client would open it again.
func TestAppsMCPLeavesNoRequestOpen(t *testing.T) {
	token := routinesClientToken(t)
	served := &inFlight{n: atomic.Int32{}, next: testApp.Routes()}
	ts := httptest.NewServer(served)
	t.Cleanup(ts.Close)

	//nolint:exhaustruct // only the list_changed handlers matter
	session := mcpClientSession(t, ts.URL+appsMCPPath, token, &mcp.ClientOptions{
		ToolListChangedHandler:     noopListChanged[*mcp.ToolListChangedRequest],
		PromptListChangedHandler:   noopListChanged[*mcp.PromptListChangedRequest],
		ResourceListChangedHandler: noopListChanged[*mcp.ResourceListChangedRequest],
	})
	_, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	assert.Eventually(t, func() bool { return served.n.Load() == 0 },
		answerWithin, 10*time.Millisecond, "an /apps/mcp request stayed open")
}

// Every kind of POST a client sends is answered and its response ended
// promptly, whatever the protocol version.
func TestMCPPostsEndPromptly(t *testing.T) {
	srv := newMCPServer(logging.NewNopLogger())
	//nolint:exhaustruct // the name is all the tool needs
	mcp.AddTool(srv, &mcp.Tool{Name: "echo"}, func(
		_ context.Context, _ *mcp.CallToolRequest, _ obs.NoArgs,
	) (*mcp.CallToolResult, any, error) {
		//nolint:exhaustruct // an empty result
		return &mcp.CallToolResult{}, nil, nil
	})
	//nolint:exhaustruct // production registers a resource too
	srv.AddResource(&mcp.Resource{URI: "test://guide", Name: "guide"}, func(
		context.Context, *mcp.ReadResourceRequest,
	) (*mcp.ReadResourceResult, error) {
		//nolint:exhaustruct // an empty result
		return &mcp.ReadResourceResult{}, nil
	})
	mux := http.NewServeMux()
	mux.Handle(appsMCPPath, streamableMCPHandler(logging.NewNopLogger(), srv))
	ts := httptest.NewServer(throttledApp(t).middleware().Then(mux))
	t.Cleanup(ts.Close)

	for _, tc := range []struct {
		name, version, body string
		status              int
	}{
		{"notification", "", `{"jsonrpc": "2.0",
			"method": "notifications/initialized"}`, http.StatusAccepted},
		{"response", "", `{"jsonrpc": "2.0", "id": 7, "result": {}}`,
			http.StatusAccepted},
		{"ping", "", `{"jsonrpc": "2.0", "id": 1, "method": "ping"}`, http.StatusOK},
		{"tools/call", "", `{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": {"name": "echo", "arguments": {}}}`, http.StatusOK},
		{"batch", "2025-03-26", `[{"jsonrpc": "2.0", "id": 1, "method": "ping"},
			{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}]`, http.StatusOK},
		{"initialize again", "", `{"jsonrpc": "2.0", "id": 1,
			"method": "initialize", "params": {"protocolVersion": "2025-11-25",
			"capabilities": {}, "clientInfo": {"name": "c", "version": "0"}}}`,
			http.StatusOK},
		{"server/discover", "2026-07-28", `{"jsonrpc": "2.0", "id": 1,
			"method": "server/discover", "params": {` + sep2575Meta + `}}`,
			http.StatusOK},
		{"subscriptions/listen", "2026-07-28", `{"jsonrpc": "2.0", "id": 1,
			"method": "subscriptions/listen", "params": {` + sep2575Meta + `,
			"notifications": {"toolsListChanged": true,
			"promptsListChanged": true, "resourcesListChanged": true}}}`,
			http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, err := postMCP(ts.URL+appsMCPPath, tc.version, tc.body)
			require.NoError(t, err, "the response did not end")
			assert.Equal(t, tc.status, status)
		})
	}
}

// postMCP POSTs body as a streamable-HTTP client does and reads the whole
// response, giving up after answerWithin.
func postMCP(endpoint, version, body string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), answerWithin)
	defer cancel()
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, strings.NewReader(body),
	)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if version >= "2026-07-28" {
		req.Header.Set("Mcp-Protocol-Version", version)
		req.Header.Set("Mcp-Method", methodOf(body))
	} else if version != "" {
		req.Header.Set("Mcp-Protocol-Version", version)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, err
}

// methodOf is the method of a single-request body, for the Mcp-Method header.
func methodOf(body string) string {
	_, rest, _ := strings.Cut(body, `"method": "`)
	method, _, _ := strings.Cut(rest, `"`)
	return method
}
