package dashboard

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	dashboardmcp "tools.xdoubleu.com/apps/dashboard/mcp"
)

// RegisterMCPTools exposes the Today summary as a read-only MCP tool.
func (a *Dashboard) RegisterMCPTools(srv *mcp.Server) {
	dashboardmcp.Register(srv, a.todayHandler)
}
