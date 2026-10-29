package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	obs "tools.xdoubleu.com/internal/obs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

// requestCountsByStatus totals http_request_duration_seconds_count for a
// method across all routes, keyed by the status label. It ignores the route
// label so the test doesn't depend on how the route is derived.
func requestCountsByStatus(t *testing.T, method string) map[string]int {
	t.Helper()

	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(
		rec, httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)
	require.Equal(t, http.StatusOK, rec.Code)

	re := regexp.MustCompile(
		`(?m)^http_request_duration_seconds_count\{method="` + regexp.QuoteMeta(method) +
			`",route="[^"]*",status="(\w+)"\} (\d+)$`,
	)
	counts := make(map[string]int)
	for _, m := range re.FindAllStringSubmatch(rec.Body.String(), -1) {
		n, err := strconv.Atoi(m[2])
		require.NoError(t, err)
		counts[m[1]] += n
	}
	return counts
}

// The SDK writes a successful MCP response body without ever calling
// WriteHeader, so net/http sends an implicit 200. The metric must record it
// as 2xx, not 0xx, or dashboards can't tell MCP successes from failures.
func TestMCPRequestsAreRecordedAs2xx(t *testing.T) {
	srv := newMCPServer(logging.NewNopLogger())
	//nolint:exhaustruct // the name is all the tool needs
	mcp.AddTool(srv, &mcp.Tool{Name: "ping"}, func(
		_ context.Context, _ *mcp.CallToolRequest, _ obs.NoArgs,
	) (*mcp.CallToolResult, any, error) {
		//nolint:exhaustruct // an empty result
		return &mcp.CallToolResult{}, nil, nil
	})
	mux := http.NewServeMux()
	mux.Handle(appsMCPPath, streamableMCPHandler(logging.NewNopLogger(), srv))
	ts := httptest.NewServer(throttledApp(t).middleware().Then(mux))
	t.Cleanup(ts.Close)

	before := requestCountsByStatus(t, http.MethodPost)

	// Connecting sends initialize; the call is a second POST with a body.
	session := mcpSession(t, ts.URL+appsMCPPath, "")
	//nolint:exhaustruct // the name is all the tool needs
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "ping"})
	require.NoError(t, err)
	require.False(t, res.IsError, toolMessage(res, err))

	// A request is observed after its handler returns, which a streaming client
	// can outrun; Close waits for every in-flight request.
	_ = session.Close()
	ts.Close()

	after := requestCountsByStatus(t, http.MethodPost)
	assert.Equal(t, before["0xx"], after["0xx"], "no request may land in 0xx")
	assert.GreaterOrEqual(t, after["2xx"]-before["2xx"], 2,
		"initialize and tools/call are both 2xx")
}
