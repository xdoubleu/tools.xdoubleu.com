package main

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/internal/mcptools"
)

// The apps MCP server exposes each app's read-only RPCs, plus the admin
// observability signals, to a local Claude CLI over streamable-HTTP, so
// production domain data and system health can be pulled in as context for
// testing changes. Every tool is gated either by the caller's own per-app
// access (mcptools.RequireAppAccess) or, for observability, by admin access
// (requireAdmin). Every app-provided tool wraps a read handler; the one
// exception is resolve_sentry_issue, an admin-gated mutation letting an
// agent close out a Sentry issue it just filed a fix for. Every tool reuses
// the same OAuth 2.1 resource-server plumbing: the api is both the resource
// server and, via the embedded internal/oauth2as provider (issue #1039),
// the authorization server — no external Auth provider involved.

const (
	appsMCPServerName = "tools-apps"

	appsMCPPath = "/apps/mcp"
	// appsResourceMetadataPath is the resource-scoped RFC 9728 metadata document
	// referenced from the apps endpoint's WWW-Authenticate challenge.
	appsResourceMetadataPath = "/.well-known/oauth-protected-resource/apps/mcp"
)

// windowArgs is the input for the two windowed observability stats tools;
// noArgs is the empty input for the rest. Both are structs so their inferred
// JSON schema is an object, as the MCP spec requires.
type windowArgs struct {
	WindowDays int32 `json:"window_days,omitempty" jsonschema:"days to look back"`
}

type noArgs struct{}

// resolveSentryIssueArgs is the input for resolve_sentry_issue — the one
// mutating observability tool, deliberately exempted from the read-only rule
// below so an agent triaging Sentry issues can close them out directly.
type resolveSentryIssueArgs struct {
	IssueID string `json:"issue_id" jsonschema:"Sentry issue ID, from get_sentry_issues"`
}

// deployLogsArgs is the input for get_deploy_logs. DeploymentID empty means
// "the latest deployment"; TailLines 0 takes the server's default backlog.
type deployLogsArgs struct {
	DeploymentID string `json:"deployment_id,omitempty" jsonschema:"optional, latest"`
	TailLines    int    `json:"tail_lines,omitempty"    jsonschema:"optional, 1000"`
}

func (app *Application) appsResourceMetadataURL() string {
	return app.config.APIURL + appsResourceMetadataPath
}

// appsMCPRoute is the fully gated apps MCP endpoint: Bearer verification → user
// promotion → the streamable-HTTP MCP handler.
func (app *Application) appsMCPRoute() http.Handler {
	return app.mcpBearerRoute(app.appsResourceMetadataURL(), app.appsMCPHandler())
}

func (app *Application) appsMCPHandler() http.Handler {
	srv := app.newAppsMCPServer()
	// DisableLocalhostProtection: the go-sdk's default DNS-rebinding guard
	// 403s any request whose accepted-connection local address is loopback
	// but whose Host header isn't. This deploy never puts api behind a
	// loopback proxy — kamal-proxy reaches it over the Docker bridge
	// network (config/deploy.api.yml), not 127.0.0.1 — so the guard
	// wouldn't fire here regardless of this flag. Disabled anyway rather
	// than relying on that distinction, since the real security boundary
	// for this endpoint is the Bearer-token check in mcpBearerRoute, which
	// already wraps this handler and doesn't care how the request arrived.
	//nolint:exhaustruct // only Stateless/DisableLocalhostProtection are set
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true},
	)
}

// newAppsMCPServer builds one MCP server: every app that implements
// MCPToolProvider contributes its read-only tools, plus the admin observability
// tools registered directly below (11 tools, which include the one mutating
// tool, resolve_sentry_issue — see registerObservabilityMCPTools).
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

// registerObservabilityMCPTools registers the 13 admin observability tools —
// 12 read-only plus resolve_sentry_issue, the one deliberate mutation. Each
// wraps a shared internal ObservabilityService method also used by the
// Connect handlers.
func registerObservabilityMCPTools(srv *mcp.Server, app *Application) {
	h := &obsConnectHandler{app: app}

	addObsTool(srv, "get_job_stats",
		"Background job run statistics and recent runs (global.job_runs).",
		func(ctx context.Context, a windowArgs) (proto.Message, error) {
			return h.jobStats(ctx, a.WindowDays)
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
		"Total database size and per-schema sizes (live pg_* queries).",
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
	addObsTool(srv, "resolve_sentry_issue",
		"Marks a Sentry issue as resolved. The one mutating observability tool.",
		func(ctx context.Context, a resolveSentryIssueArgs) (proto.Message, error) {
			return h.resolveSentryIssue(ctx, a.IssueID)
		})
	addObsTool(srv, "get_deploy_status",
		"Phase and health of the latest DigitalOcean deployment.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.deployStatus(ctx), nil
		})
	addObsTool(
		srv,
		"get_deploy_logs",
		"Build/deploy/runtime log text for a DigitalOcean deployment (default: "+
			"the latest). Runtime logs come from the app's active deployment.",
		func(ctx context.Context, a deployLogsArgs) (proto.Message, error) {
			return h.deployLogs(ctx, a.DeploymentID, a.TailLines), nil
		},
	)
}

// addObsTool registers one read-only observability tool. It applies the admin
// gate uniformly and marshals the shared method's proto response to JSON text
// content, so the tool bodies stay a thin wrapper over the ObservabilityService
// read methods.
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
