package dashboard

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	dashboardv1 "tools.xdoubleu.com/gen/dashboard/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "dashboard"

// RegisterMCPTools exposes the Today summary as a read-only MCP tool.
func (a *Dashboard) RegisterMCPTools(srv *mcp.Server) {
	h := &todayConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "dashboard_get_today",
		"What needs the user's attention today across the apps they can use: "+
			"books being read, unread feed items, today's meals, the shopping "+
			"list size, titles being watched, active learning paths and tasks "+
			"waiting on them.",
		func(ctx context.Context, _ mcptools.NoArgs) (proto.Message, error) {
			return h.GetToday(ctx, &dashboardv1.GetTodayRequest{})
		})
}
