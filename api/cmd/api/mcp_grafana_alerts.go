package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// grafanaQueryTimeout bounds one call to Grafana's HTTP API. Unlike
// prom_query's Prometheus (an internal-network accessory), Grafana is a
// Kamal service reached over the public host through kamal-proxy
// (config/deploy.grafana.yml) — Kamal gives its app containers no stable
// network alias, the lesson of issue #1554 (infra/prometheus.yml) — so this
// is a real outbound HTTPS round trip.
const grafanaQueryTimeout = 10 * time.Second

// grafanaAdminUser is the username get_grafana_alerts authenticates as.
// config/deploy.grafana.yml sets no GF_SECURITY_ADMIN_USER, so Grafana's
// default "admin" account is the one GF_SECURITY_ADMIN_PASSWORD unlocks.
const grafanaAdminUser = "admin"

// grafanaRulesPath is Grafana's Prometheus-compatible ruler endpoint: it
// carries each Grafana-managed alert rule's current `state` and its
// `alerts[]` active-instance list, which /api/v1/provisioning/alert-rules
// (rule definitions only) does not.
const grafanaRulesPath = "/api/prometheus/grafana/api/v1/rules"

// errGrafanaNotConfigured is returned when GRAFANA_ADMIN_PASSWORD is unset —
// there is then no credential to authenticate to Grafana's API with, so the
// tool cannot run (local development, or a deploy that never added the
// secret).
var errGrafanaNotConfigured = errors.New(
	"GRAFANA_ADMIN_PASSWORD not configured",
)

// registerGrafanaAlertsMCPTool registers get_grafana_alerts — the read path
// for Grafana-managed alert-rule state. All alerting moved to Grafana in
// issue #1528, and Grafana-managed alerts never appear in Prometheus
// ALERTS{}, so prom_query cannot answer "is the alert for X firing?" (issue
// #1564). Like prom_query, it proxies the upstream JSON straight through
// rather than re-modeling it into a proto message this repo defines.
func registerGrafanaAlertsMCPTool(srv *mcp.Server, app *Application) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_grafana_alerts",
		Description: "Returns every provisioned Grafana alert rule's current " +
			"state (Normal/Pending/Alerting/NoData/Error) plus its labels, " +
			"annotations and active instances, from Grafana's " +
			"Prometheus-compatible ruler API. All alerting is Grafana-managed " +
			"(issue #1528) and Grafana-managed alerts never appear in " +
			"Prometheus ALERTS{}, so prom_query cannot confirm or investigate " +
			"a firing alert — this tool can. Returns Grafana's raw API JSON " +
			"({\"status\":\"success\",\"data\":{\"groups\":[...]}}).",
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		_ noArgs,
	) (*mcp.CallToolResult, any, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, nil, err
		}
		body, err := grafanaAlerts(
			ctx, app.config.GrafanaURL, app.config.GrafanaAdminPassword,
		)
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

// grafanaAlerts calls Grafana's Prometheus-compatible rules endpoint as the
// admin user and returns the raw response body — Grafana's own JSON shape is
// passed straight through, since get_grafana_alerts covers the whole rule
// set rather than a fixed query.
func grafanaAlerts(
	ctx context.Context,
	grafanaURL, adminPassword string,
) ([]byte, error) {
	if adminPassword == "" {
		return nil, errGrafanaNotConfigured
	}

	reqCtx, cancel := context.WithTimeout(ctx, grafanaQueryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		reqCtx, http.MethodGet, grafanaURL+grafanaRulesPath, nil,
	)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(grafanaAdminUser, adminPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
