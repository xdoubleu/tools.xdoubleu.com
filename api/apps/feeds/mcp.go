package feeds

import (
	"context"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/internal/mcptools"
)

const mcpAppName = "feeds"

// RegisterMCPTools exposes the feeds app's read-only RPCs on the combined
// apps MCP server. Every tool returns the calling user's own feed data.
func (a *Feeds) RegisterMCPTools(srv *mcp.Server) {
	h := &feedsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_feeds",
		"The user's RSS/Atom and email-newsletter feed subscriptions.",
		h.mcpListFeeds)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_items",
		"Items ingested by any of the user's feeds.", h.mcpListItems)
}

func (h *feedsConnectHandler) mcpListFeeds(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListFeeds(ctx, connect.NewRequest(
		&feedsv1.ListFeedsRequest{},
	)))
}

func (h *feedsConnectHandler) mcpListItems(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListFeedItems(ctx, connect.NewRequest(
		&feedsv1.ListFeedItemsRequest{},
	)))
}
