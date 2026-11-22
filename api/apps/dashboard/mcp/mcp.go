// Package mcp exposes the dashboard app's Today summary as an MCP tool.
package mcp

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/dashboard/internal/protomap"
	"tools.xdoubleu.com/apps/dashboard/internal/services"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
)

const mcpAppName = "dashboard"

type tools struct {
	today *services.TodayService
}

// Register exposes the Today summary as a read-only MCP tool.
func Register(srv *mcp.Server, today *services.TodayService) {
	t := tools{today: today}
	mcptools.AddReadTool(srv, mcpAppName, "dashboard_get_today",
		"What needs the user's attention today across the apps they can use: "+
			"books being read, unread feed items, today's meals, the shopping "+
			"list size, titles being watched, active learning paths and tasks "+
			"waiting on them.", t.getToday)
}

func (t tools) getToday(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated, "user not authenticated",
		)
	}
	return protomap.Today(t.today.Get(ctx, user)), nil
}
