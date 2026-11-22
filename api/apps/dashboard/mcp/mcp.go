// Package mcp exposes the dashboard app's Today summary as an MCP tool.
package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/dashboard/connect"
	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

const mcpAppName = "dashboard"

// Register exposes the Today summary as a read-only MCP tool.
func Register(srv *mcp.Server, h *connect.TodayHandler) {
	mcptools.AddReadTool(srv, mcpAppName, "dashboard_get_today",
		"What needs the user's attention today across the apps they can use: "+
			"books being read, unread feed items, today's meals, the shopping "+
			"list size, titles being watched, active learning paths and tasks "+
			"waiting on them.",
		func(ctx context.Context, _ mcptools.NoArgs) (proto.Message, error) {
			return h.GetToday(ctx, &dashboardv1.GetTodayRequest{})
		})
}
