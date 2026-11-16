package mcptools

import (
	"context"
	"slices"

	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/models"
)

// ObservabilityGrant is the app grant that lets an agent account call the
// observability tools in agentObservabilityTools (adr-0052). Only a routine
// job's account holds it.
const ObservabilityGrant = "observability"

// agentObservabilityTools are the observability tools a grant opens: the read
// tools and resolve_sentry_issue. dismiss_security_alert stays admin-only.
//
//nolint:gochecknoglobals // read-only allowlist
var agentObservabilityTools = []string{
	"get_database_stats", "get_failing_pull_requests", "get_grafana_alerts",
	"get_job_stats", "get_logs", "get_notification_settings",
	"get_oauth_connections", "get_project_issues_by_status",
	"get_security_alerts", "get_sentry_issue_event", "get_sentry_issues",
	"get_slow_transactions", "get_storage_stats", "get_usage_stats",
	"get_workflow_runs", "prom_query", "resolve_sentry_issue",
}

// agentMayObserve reports whether an agent holding apps may call the tool.
func agentMayObserve(apps []string, tool string) bool {
	return slices.Contains(apps, ObservabilityGrant) &&
		slices.Contains(agentObservabilityTools, tool)
}

// CanObserve reports whether the caller may use the observability tool: an
// admin, the service role, or an agent granted it.
func CanObserve(ctx context.Context, tool string) bool {
	user := contexttools.GetValue[models.User](ctx, constants.UserContextKey)
	if user == nil {
		return false
	}
	switch user.Role {
	case models.RoleAdmin, models.RoleService:
		return true
	case models.RoleAgent:
		return agentMayObserve(user.AppAccess, tool)
	case models.RoleUser:
		return false
	}
	return false
}
