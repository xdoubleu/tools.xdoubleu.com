package main

import (
	"context"
	"encoding/json"
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

// grafanaAlertsArgs are get_grafana_alerts's optional arguments.
type grafanaAlertsArgs struct {
	// RuleName, when set, restricts the response to the one rule with this
	// exact name — the common "is alert X firing?" check, which otherwise
	// pays for every provisioned rule's full state in one response.
	RuleName string `json:"rule_name,omitempty"`
}

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
		Description: "Returns provisioned Grafana alert rules' current state " +
			"(Normal/Pending/Alerting/NoData/Error) plus their labels, " +
			"annotations and active instances, from Grafana's " +
			"Prometheus-compatible ruler API. All alerting is Grafana-managed " +
			"(issue #1528) and Grafana-managed alerts never appear in " +
			"Prometheus ALERTS{}, so prom_query cannot confirm or investigate " +
			"a firing alert — this tool can. Pass rule_name to check one rule " +
			"(\"is alert X firing?\") instead of pulling every rule's state. " +
			"Returns Grafana's raw API JSON " +
			"({\"status\":\"success\",\"data\":{\"groups\":[...]}}), filtered " +
			"to groups containing the requested rule when rule_name is set.",
	}, func(
		ctx context.Context,
		_ *mcp.CallToolRequest,
		args grafanaAlertsArgs,
	) (*mcp.CallToolResult, any, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, nil, err
		}
		body, err := grafanaAlerts(
			ctx, app.config.GrafanaURL, app.config.GrafanaAdminPassword,
		)
		if err == nil && args.RuleName != "" {
			body, err = filterGrafanaRules(body, args.RuleName)
		}
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

// filterGrafanaRules keeps only the rule groups containing the rule named
// ruleName, preserving Grafana's response envelope and everything inside the
// surviving groups. A name that matches no rule yields an empty groups list
// rather than an error — "no such rule" is a normal answer, not a failure.
// The filtering happens after the fetch (the upstream endpoint offers no
// server-side rule filter), so the cost saved is response size, not the
// round trip.
func filterGrafanaRules(body []byte, ruleName string) ([]byte, error) {
	// Groups stay as raw JSON — the filter only reads each group's rule
	// names to decide membership, and re-emits the group untouched.
	var payload struct {
		Data struct {
			Groups []json.RawMessage `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	filtered := make([]json.RawMessage, 0, len(payload.Data.Groups))
	for _, group := range payload.Data.Groups {
		var rules struct {
			Rules []struct {
				Name string `json:"name"`
			} `json:"rules"`
		}
		if err := json.Unmarshal(group, &rules); err != nil {
			return nil, err
		}
		for _, rule := range rules.Rules {
			if rule.Name == ruleName {
				filtered = append(filtered, group)
				break
			}
		}
	}

	// Re-marshal through a generic map so any envelope fields the filter
	// struct doesn't model survive the round trip untouched.
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		data = map[string]any{}
		envelope["data"] = data
	}
	data["groups"] = filtered

	return json.Marshal(envelope)
}
