package main

import (
	"log/slog"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tools.xdoubleu.com/internal/mcptools"
)

// streamableMCPHandler serves srv statelessly over streamable HTTP, logging
// slow requests. The real boundary is mcpBearerRoute's token check, so the
// go-sdk's loopback DNS-rebinding guard is disabled (kamal-proxy doesn't
// reach us over loopback anyway).
func streamableMCPHandler(logger *slog.Logger, srv *mcp.Server) http.Handler {
	//nolint:exhaustruct // only Stateless/DisableLocalhostProtection are set
	return mcptools.LogSlowRequests(logger, mcptools.KeepRequestDeadline(
		mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return srv },
			&mcp.StreamableHTTPOptions{Stateless: true, DisableLocalhostProtection: true},
		),
	))
}

// newMCPServer is an MCP server whose tool calls keep their request's
// deadline and are logged when slow.
func newMCPServer(logger *slog.Logger) *mcp.Server {
	//nolint:exhaustruct // only Name/Version identify the server
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    appsMCPServerName,
		Version: mcpServerVersion,
	}, &mcp.ServerOptions{Capabilities: staticListCapabilities()})
	srv.AddReceivingMiddleware(mcptools.BoundToolCalls(logger))
	return srv
}

// staticListCapabilities keeps the SDK's default logging capability but
// advertises no list_changed: tools and resources are registered once, at
// startup. Advertised, it makes a 2026-07-28 client hold a
// subscriptions/listen POST open until the request budget cuts it, then
// reopen it.
func staticListCapabilities() *mcp.ServerCapabilities {
	//nolint:exhaustruct // only the capabilities this server has
	return &mcp.ServerCapabilities{
		//nolint:staticcheck // SA1019: the SDK default, kept for legacy clients
		Logging:   &mcp.LoggingCapabilities{},
		Tools:     &mcp.ToolCapabilities{ListChanged: false},
		Resources: &mcp.ResourceCapabilities{ListChanged: false, Subscribe: false},
	}
}
