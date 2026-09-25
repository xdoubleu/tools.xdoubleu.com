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

// grafanaQueryTimeout bounds one Grafana API call. Grafana is reached over the
// public host via kamal-proxy (Kamal gives no stable network alias), so this
// is a real HTTPS round trip.
const grafanaQueryTimeout = 10 * time.Second

// grafanaAdminUser is Grafana's default admin, unlocked by
// GF_SECURITY_ADMIN_PASSWORD.
const grafanaAdminUser = "admin"

// grafanaRulesPath is the ruler endpoint, which (unlike provisioning) carries
// each rule's current state and active alerts.
const grafanaRulesPath = "/api/prometheus/grafana/api/v1/rules"

// errGrafanaNotConfigured is returned when GRAFANA_ADMIN_PASSWORD is unset.
var errGrafanaNotConfigured = errors.New(
	"GRAFANA_ADMIN_PASSWORD not configured",
)

// grafanaAlertsArgs are get_grafana_alerts's optional arguments.
type grafanaAlertsArgs struct {
	// RuleName restricts the response to the rule with this exact name.
	RuleName string `json:"rule_name,omitempty"`
}

// registerGrafanaAlertsMCPTool registers get_grafana_alerts. Grafana-managed
// alerts never appear in Prometheus ALERTS{}, so prom_query can't answer
// "is X firing?". Proxies Grafana's JSON unchanged.
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

// grafanaAlerts returns the rules endpoint's raw body.
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

// filterGrafanaRules keeps only groups containing ruleName, preserving the
// envelope. No match yields empty groups, not an error. Filtering is
// client-side; the endpoint has no rule filter.
func filterGrafanaRules(body []byte, ruleName string) ([]byte, error) {
	// Groups stay raw JSON and are re-emitted untouched.
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

	// Round-trip through a map so unmodeled envelope fields survive.
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
