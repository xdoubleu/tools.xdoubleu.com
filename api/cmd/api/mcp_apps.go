package main

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	platformmcp "tools.xdoubleu.com/internal/platform/mcp"
	"tools.xdoubleu.com/internal/platform/obs"
)

// The apps MCP server exposes each app's read RPCs plus admin observability
// tools over streamable HTTP. Tools are gated by per-app access or
// obs.requireObservability. The observability tools live in internal/platform/obs.
// Deliberate mutations: learningpaths' write tools (docs/adr-0023) and the
// admin tools resolve_sentry_issue, dismiss_security_alert and record_action.

const (
	appsMCPServerName = "tools-apps"

	appsMCPPath = "/apps/mcp"
	// appsResourceMetadataPath is the resource-scoped RFC 9728 metadata document.
	appsResourceMetadataPath = "/.well-known/oauth-protected-resource/apps/mcp"
)

func (app *Application) appsResourceMetadataURL() string {
	return app.config.APIURL + appsResourceMetadataPath
}

// appsMCPRoute is the gated apps MCP endpoint.
func (app *Application) appsMCPRoute() http.Handler {
	return app.mcpResource().BearerRoute(
		app.appsResourceMetadataURL(), app.appsMCPHandler(),
	)
}

func (app *Application) appsMCPHandler() http.Handler {
	return platformmcp.StreamableHandler(app.logger, app.newAppsMCPServer())
}

// newAppsMCPServer builds the MCP server from every MCPToolProvider plus the
// observability tools.
func (app *Application) newAppsMCPServer() *mcp.Server {
	srv := platformmcp.NewServer(app.logger, appsMCPServerName)
	for _, a := range *app.apps {
		if provider, ok := a.(MCPToolProvider); ok {
			provider.RegisterMCPTools(srv)
		}
	}
	obs.RegisterMCPTools(srv, app.obsDeps())

	return srv
}

// mcpResource is the MCP resource server for the embedded authorization server.
func (app *Application) mcpResource() platformmcp.Resource {
	return platformmcp.Resource{
		Tokens: app.auth,
		APIURL: app.config.APIURL,
		Issuer: app.config.AuthIssuer,
	}
}
