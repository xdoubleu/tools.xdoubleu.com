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
// production domain data and system health can be pulled in as read-only
// context for testing changes. Every tool wraps an existing read handler and
// is gated either by the caller's own per-app access
// (mcptools.RequireAppAccess) or, for observability, by admin access
// (requireAdmin) — never a write. Every tool reuses the same OAuth 2.1
// resource-server plumbing: the api is the resource server, Supabase is the
// authorization server.

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
	//nolint:exhaustruct // Stateless is the only option this read-only server sets
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)
}

// newAppsMCPServer builds one MCP server: every app that implements
// MCPToolProvider contributes its read-only tools, plus the admin observability
// tools registered directly below. Apps register tools against their own
// (unexported) read handlers, so the query + proto mapping stays in the app
// package and no write RPC is reachable.
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

// registerObservabilityMCPTools registers the 8 read-only admin observability
// tools. Each wraps a shared internal ObservabilityService read method — no
// write RPC is reachable, so the tools are read-only by construction.
func registerObservabilityMCPTools(srv *mcp.Server, app *Application) {
	h := &obsConnectHandler{app: app}

	addObsTool(srv, "get_job_stats",
		"Background job run statistics and recent runs (global.job_runs).",
		func(ctx context.Context, a windowArgs) (proto.Message, error) {
			return h.jobStats(ctx, a.WindowDays)
		})
	addObsTool(srv, "get_usage_stats",
		"Per-day request counts by app and endpoint (global.usage_daily).",
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
	addObsTool(srv, "get_github_issues",
		"Open GitHub issues for the repository.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.githubIssues(ctx), nil
		})
	addObsTool(srv, "get_failing_pull_requests",
		"Open pull requests with at least one failing CI check.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.failingPullRequests(ctx), nil
		})
	addObsTool(srv, "get_sentry_issues",
		"Unresolved Sentry issues for the project.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.sentryIssues(ctx), nil
		})
	addObsTool(srv, "get_deploy_status",
		"Phase and health of the latest DigitalOcean deployment.",
		func(ctx context.Context, _ noArgs) (proto.Message, error) {
			return h.deployStatus(ctx), nil
		})
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
