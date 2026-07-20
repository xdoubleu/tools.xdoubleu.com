package main

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The apps MCP server exposes each app's read-only RPCs to a local Claude CLI
// over streamable-HTTP, so production domain data can be pulled in as read-only
// context for testing changes. Every tool wraps an existing read handler and is
// gated by the caller's own per-app access (mcptools.RequireAppAccess), so it
// grants exactly what the signed-in user already has — never a write. It reuses
// the same OAuth 2.1 resource-server plumbing as the observability server: the
// api is the resource server, Supabase is the authorization server.

const (
	appsMCPServerName = "tools-apps"

	appsMCPPath = "/apps/mcp"
	// appsResourceMetadataPath is the resource-scoped RFC 9728 metadata document
	// referenced from the apps endpoint's WWW-Authenticate challenge.
	appsResourceMetadataPath = "/.well-known/oauth-protected-resource/apps/mcp"
)

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

// newAppsMCPServer builds one MCP server and lets every app that implements
// MCPToolProvider contribute its read-only tools. Apps register tools against
// their own (unexported) read handlers, so the query + proto mapping stays in
// the app package and no write RPC is reachable.
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

	return srv
}
