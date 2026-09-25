package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// promQueryTimeout bounds one instant query so a stuck Prometheus doesn't hang
// the MCP call.
const promQueryTimeout = 10 * time.Second

// registerPromQueryMCPTool registers prom_query, proxying Prometheus's
// /api/v1/query directly since the response isn't a proto message.
func registerPromQueryMCPTool(srv *mcp.Server, app *Application) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{
		Name: "prom_query",
		Description: "Runs a PromQL instant query against Prometheus (host " +
			"CPU/memory/disk from node_exporter, Postgres stats from " +
			"postgres_exporter, api's own /metrics) and returns Prometheus's " +
			"raw API v1 query response JSON. Replaces the old get_host_metrics/" +
			"get_database_size_history/get_transaction_latency_history/" +
			"get_alert_states tools now that Grafana + Prometheus own that " +
			"data (issue #1468). Examples: " +
			`'node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes', ` +
			`'rate(pg_stat_database_xact_commit[5m])', 'up{job="api"}'.`,
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args promQueryArgs,
	) (*mcp.CallToolResult, any, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, nil, err
		}
		body, err := promQuery(ctx, app.config.PrometheusURL, args.Query)
		if err != nil {
			return nil, nil, err
		}
		//nolint:exhaustruct // only Content carries the tool output
		return &mcp.CallToolResult{
			//nolint:exhaustruct // TextContent needs only Text
			Content: []mcp.Content{&mcp.TextContent{Text: string(body)}},
		}, nil, nil
	})
}

// promQuery returns Prometheus's raw instant-query JSON.
func promQuery(ctx context.Context, prometheusURL, query string) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, promQueryTimeout)
	defer cancel()

	endpoint := prometheusURL + "/api/v1/query?" +
		url.Values{"query": {query}}.Encode()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
