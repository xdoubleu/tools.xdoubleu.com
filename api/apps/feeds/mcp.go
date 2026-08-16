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

type mcpListItemsArgs struct {
	FeedID     string `json:"feed_id,omitempty"     jsonschema:"restrict to one feed"`
	Limit      int32  `json:"limit,omitempty"       jsonschema:"page size, 0 for default"`
	Offset     int32  `json:"offset,omitempty"      jsonschema:"page offset"`
	UnreadOnly bool   `json:"unread_only,omitempty" jsonschema:"exclude read items"`
}

// RegisterMCPTools exposes the feeds app's read-only RPCs on the combined
// apps MCP server. Every tool returns the calling user's own feed data.
func (a *Feeds) RegisterMCPTools(srv *mcp.Server) {
	h := &feedsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_feeds",
		"The user's RSS/Atom and email-newsletter feed subscriptions.",
		h.mcpListFeeds)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_items",
		"Items ingested by any of the user's feeds. Article bodies are "+
			"omitted — use feeds_get_item for one item's content.",
		h.mcpListItems)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_get_item",
		"One feed item including its extracted article body.", h.mcpGetItem)
}

type mcpGetItemArgs struct {
	ItemID string `json:"item_id" jsonschema:"the item's id"`
}

func (h *feedsConnectHandler) mcpListFeeds(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListFeeds(ctx, connect.NewRequest(
		&feedsv1.ListFeedsRequest{},
	)))
}

func (h *feedsConnectHandler) mcpListItems(
	ctx context.Context, args mcpListItemsArgs,
) (proto.Message, error) {
	req := &feedsv1.ListFeedItemsRequest{
		Limit:      args.Limit,
		Offset:     args.Offset,
		UnreadOnly: proto.Bool(args.UnreadOnly),
	}
	if args.FeedID != "" {
		req.FeedId = proto.String(args.FeedID)
	}
	return mcptools.Unwrap(h.ListFeedItems(ctx, connect.NewRequest(req)))
}

func (h *feedsConnectHandler) mcpGetItem(
	ctx context.Context, args mcpGetItemArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetFeedItem(ctx, connect.NewRequest(
		&feedsv1.GetFeedItemRequest{ItemId: args.ItemID},
	)))
}
