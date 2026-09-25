package main

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/mcptools"
)

// The apps MCP server exposes each app's read RPCs plus admin observability
// tools over streamable HTTP. Tools are gated by per-app access or requireAdmin.
// Deliberate mutations: learningpaths' write tools (docs/adr-0023) and the
// admin tools resolve_sentry_issue, dismiss_security_alert, record_action and
// notify_slack.

const (
	appsMCPServerName = "tools-apps"

	appsMCPPath = "/apps/mcp"
	// appsResourceMetadataPath is the resource-scoped RFC 9728 metadata document.
	appsResourceMetadataPath = "/.well-known/oauth-protected-resource/apps/mcp"
)

// windowArgs/noArgs are structs so their JSON schema is an object, as MCP
// requires.
type windowArgs struct {
	WindowDays int32 `json:"window_days,omitempty" jsonschema:"days to look back"`
}

type noArgs struct{}

// resolveSentryIssueArgs is the input for resolve_sentry_issue.
type resolveSentryIssueArgs struct {
	IssueID string `json:"issue_id" jsonschema:"Sentry issue ID, from get_sentry_issues"`
}

// dismissSecurityAlertArgs: AlertType is "dependabot", "code_scanning" or
// "secret_scanning". Valid Reasons per type:
// dependabot: fix_started|inaccurate|no_bandwidth|not_used|tolerable_risk.
// code_scanning: "false positive"|"won't fix"|"used in tests".
// secret_scanning: false_positive|wont_fix|revoked|used_in_tests|pattern_deleted.
type dismissSecurityAlertArgs struct {
	AlertType   string `json:"alert_type"   jsonschema:"see this type's doc comment"`
	AlertNumber int64  `json:"alert_number" jsonschema:"see get_security_alerts"`
	Reason      string `json:"reason"       jsonschema:"see this type's doc comment"`
}

// recordActionArgs: a routine calls "open" first (TriggerSource
// schedule|api|manual, RoutineName) and "close" last (the returned ID,
// Outcome succeeded|failed|no_action_needed, optional PRURL/Error).
type recordActionArgs struct {
	Mode          string `json:"mode"                     jsonschema:"open or close"`
	TriggerSource string `json:"trigger_source,omitempty" jsonschema:"see doc comment"`
	RoutineName   string `json:"routine_name,omitempty"   jsonschema:"see doc comment"`
	ID            int64  `json:"id,omitempty"             jsonschema:"see doc comment"`
	Outcome       string `json:"outcome,omitempty"        jsonschema:"see doc comment"`
	PRURL         string `json:"pr_url,omitempty"         jsonschema:"see doc comment"`
	Error         string `json:"error,omitempty"          jsonschema:"see doc comment"`
}

// notifySlackArgs: Title, if set, is bolded above Message.
type notifySlackArgs struct {
	Message string `json:"message"         jsonschema:"the summary to post to Slack"`
	Title   string `json:"title,omitempty" jsonschema:"optional bolded title line"`
}

// projectIssuesByStatusArgs is the input for get_project_issues_by_status.
type projectIssuesByStatusArgs struct {
	ProjectNumber int32  `json:"project_number,omitempty" jsonschema:"board number"`
	Status        string `json:"status,omitempty"         jsonschema:"e.g. Ready"`
}

// promQueryArgs is the input for prom_query.
type promQueryArgs struct {
	Query string `json:"query" jsonschema:"a PromQL expression, e.g. up{job='api'}"`
}

// logsArgs is the input for get_logs. Source/MinLevel empty means "any".
type logsArgs struct {
	Source   string `json:"source,omitempty"    jsonschema:"api or web, optional"`
	MinLevel string `json:"min_level,omitempty" jsonschema:"optional"`
	Since    string `json:"since,omitempty"     jsonschema:"RFC3339, optional"`
}

func (app *Application) appsResourceMetadataURL() string {
	return app.config.APIURL + appsResourceMetadataPath
}

// appsMCPRoute is the gated apps MCP endpoint.
func (app *Application) appsMCPRoute() http.Handler {
	return app.mcpBearerRoute(app.appsResourceMetadataURL(), app.appsMCPHandler())
}

func (app *Application) appsMCPHandler() http.Handler {
	srv := app.newAppsMCPServer()
	// The real boundary is mcpBearerRoute's token check, so the go-sdk's
	// loopback DNS-rebinding guard is disabled (kamal-proxy doesn't reach us over
	// loopback anyway).
	//nolint:exhaustruct // only Stateless/DisableLocalhostProtection are set
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true},
	)
}

// newAppsMCPServer builds the MCP server from every MCPToolProvider plus the
// observability tools.
func (app *Application) newAppsMCPServer() *mcp.Server {
	//nolint:exhaustruct // only Name/Version identify the server
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    appsMCPServerName,
		Version: mcpServerVersion,
	}, nil)

	for _, a := range *app.apps {
		if provider, ok := a.(MCPToolProvider); ok {
			provider.RegisterMCPTools(srv)
		}
	}
	registerObservabilityMCPTools(srv, app)

	return srv
}

// registerObservabilityMCPTools registers the admin tools. Most wrap shared
// ObservabilityService methods; prom_query and get_grafana_alerts proxy
// Prometheus/Grafana directly.
func registerObservabilityMCPTools(srv *mcp.Server, app *Application) {
	h := &obsConnectHandler{app: app}

	addObsTool(srv, "get_job_stats",
		"Background job run statistics and recent runs (global.job_runs).",
		func(ctx context.Context, a windowArgs) (proto.Message, error) {
			return h.jobStats(ctx, a.WindowDays)
		})
	addObsTool(srv, "get_automated_actions",
		"Run history for self-healing routines (global.automated_actions) — "+
			"distinct from get_job_stats: these run outside api's own process "+
			"(e.g. on scheduled-agent infrastructure), so a row only exists "+
			"because the routine itself opened and closed it via record_action.",
		func(ctx context.Context, a windowArgs) (proto.Message, error) {
			return h.automatedActions(ctx, a.WindowDays)
		})
	addObsTool(srv, "get_usage_stats",
		"Per-day request counts and response bytes by app and endpoint "+
			"(global.usage_daily). Bytes measure what left the api, a proxy "+
			"for what it read out of the database — sort by it to find the "+
			"endpoints driving database egress.",
		func(ctx context.Context, a windowArgs) (proto.Message, error) {
			return h.usageStats(ctx, a.WindowDays)
		})
	addObsTool(srv, "get_storage_stats",
		"Latest R2 object-store snapshot plus recent history.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.storageStats(ctx)
		})
	addObsTool(srv, "get_database_stats",
		"Total database size and per-schema sizes (live pg_* queries). "+
			"Growth-over-time now lives in Grafana/Prometheus — use prom_query "+
			"(e.g. pg_database_size_bytes via postgres_exporter) for a trend.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.databaseStats(ctx)
		})
	addObsTool(srv, "get_failing_pull_requests",
		"Open pull requests with at least one failing CI check.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.failingPullRequests(ctx), nil
		})
	addObsTool(srv, "get_workflow_runs",
		"Recent pull-request and push (main branch) GitHub Actions workflow "+
			"runs, with duration for each completed run.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.workflowRuns(ctx), nil
		})
	addObsTool(srv, "get_security_alerts",
		"Open GitHub security alerts: Dependabot (dependencies), code "+
			"scanning (CodeQL/SARIF findings), and secret scanning (leaked "+
			"credentials).",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.securityAlerts(ctx), nil
		})
	addObsTool(srv, "get_oauth_connections",
		"Connection state of each external provider (GitHub, Sentry, "+
			"DigitalOcean), with the scopes each connection was authorized "+
			"with, the scopes the provider echoed back, and the scopes "+
			"required today — the three that explain a provider reporting "+
			"itself not connected.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.oauthConnections(ctx)
		})
	addObsTool(srv, "get_sentry_issues",
		"Unresolved Sentry issues for the project.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.sentryIssues(ctx), nil
		})
	addObsTool(srv, "get_slow_transactions",
		"Slow API endpoints/pages: currently-slowest transactions (live from "+
			"Sentry) plus ones regressing over time (from stored history).",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.slowTransactions(ctx)
		})
	registerMutatingObservabilityMCPTools(srv, h)
	registerAlertMCPTools(srv, h)
}

// registerMutatingObservabilityMCPTools registers the four mutating tools
// (split out for the function-length lint).
func registerMutatingObservabilityMCPTools(srv *mcp.Server, h *obsConnectHandler) {
	addObsTool(srv, "resolve_sentry_issue",
		"Marks a Sentry issue as resolved. One of four mutating observability "+
			"tools, alongside dismiss_security_alert, record_action, and "+
			"notify_slack.",
		func(ctx context.Context, a resolveSentryIssueArgs) (proto.Message, error) {
			return h.resolveSentryIssue(ctx, a.IssueID)
		})
	addObsTool(srv, "dismiss_security_alert",
		"Dismisses/resolves a GitHub Dependabot, code-scanning, or "+
			"secret-scanning security alert. One of four mutating observability "+
			"tools, alongside resolve_sentry_issue, record_action, and "+
			"notify_slack.",
		func(ctx context.Context, a dismissSecurityAlertArgs) (proto.Message, error) {
			return h.dismissSecurityAlert(
				ctx, github.SecurityAlertType(a.AlertType), a.AlertNumber, a.Reason,
			)
		})
	addObsTool(srv, "record_action",
		"Opens or closes a run record for a self-healing routine "+
			"(global.automated_actions) — a routine calls this with mode=open "+
			"as its first step and mode=close as its last, since it executes "+
			"outside api's own process and nothing else observes it running. "+
			"One of four mutating observability tools, alongside "+
			"resolve_sentry_issue, dismiss_security_alert, and notify_slack.",
		func(ctx context.Context, a recordActionArgs) (proto.Message, error) {
			return h.recordAction(ctx, a)
		})
	addObsTool(srv, "notify_slack",
		"Posts a summary to a configured Slack Incoming Webhook — used to "+
			"announce an epic-complete summary from either a local Claude Code "+
			"session or Claude Code on the web, since the send happens "+
			"server-side. One of four mutating observability tools, alongside "+
			"resolve_sentry_issue, dismiss_security_alert, and record_action.",
		func(ctx context.Context, a notifySlackArgs) (proto.Message, error) {
			return h.notifySlack(ctx, a.Title, a.Message)
		})
}

// registerAlertMCPTools registers get_logs, get_notification_settings and
// prom_query (split out for the function-length lint).
func registerAlertMCPTools(srv *mcp.Server, h *obsConnectHandler) {
	addObsTool(srv, "get_logs",
		"Application logs forwarded from api and web, optionally filtered by "+
			"source/level.",
		func(ctx context.Context, a logsArgs) (proto.Message, error) {
			return h.logs(ctx, a.Source, a.MinLevel, a.Since)
		})
	addObsTool(srv, "get_notification_settings",
		"Per-source enabled/disabled state of the email notifications "+
			"WeeklyDigestJob sends (unhealthy_feeds, open_feed_items) — "+
			"explains why an expected notification email didn't go out.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.notificationSettings(ctx)
		})
	registerPromQueryMCPTool(srv, h.app)
	registerGrafanaAlertsMCPTool(srv, h.app)
	addObsTool(srv, "get_project_issues_by_status",
		"Open issues on the configured repository owner's GitHub Projects "+
			"(v2) board whose Status column matches the given name (e.g. "+
			"\"Ready\"). The generic GitHub MCP server can't resolve custom "+
			"fields on a personal (user-owned) project board, so this is the "+
			"only way to answer \"which issues are in column X\".",
		func(ctx context.Context, a projectIssuesByStatusArgs) (proto.Message, error) {
			return h.projectIssuesByStatus(ctx, a.ProjectNumber, a.Status), nil
		})
}

// addObsTool registers a read-only observability tool: admin gate, then the
// proto response marshalled to JSON text.
func addObsTool[In any](
	srv *mcp.Server,
	name, description string,
	produce func(context.Context, In) (proto.Message, error),
) {
	//nolint:exhaustruct // name/description are the only fields tools need
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: description},
		func(
			ctx context.Context,
			_ *mcp.CallToolRequest,
			args In,
		) (*mcp.CallToolResult, any, error) {
			if err := requireAdmin(ctx); err != nil {
				return nil, nil, err
			}
			msg, err := produce(ctx, args)
			if err != nil {
				return nil, nil, err
			}
			return mcptools.Result(msg)
		})
}
