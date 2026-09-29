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

//nolint:lll // struct tags can't wrap
type mcpListItemsArgs struct {
	FeedID       string `json:"feed_id,omitempty"       jsonschema:"restrict to one feed"`
	Limit        int32  `json:"limit,omitempty"         jsonschema:"page size, 0 for default"`
	Offset       int32  `json:"offset,omitempty"        jsonschema:"page offset"`
	UnreadOnly   bool   `json:"unread_only,omitempty"   jsonschema:"exclude read items"`
	FilteredOnly bool   `json:"filtered_only,omitempty" jsonschema:"only items filter rules hide, with the rule; ignores unread_only"`
}

// RegisterMCPTools exposes the app's read-only RPCs as MCP tools, scoped to
// the caller.
func (a *Feeds) RegisterMCPTools(srv *mcp.Server) {
	h := &feedsConnectHandler{app: a}

	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_feeds",
		"The user's RSS/Atom and email-newsletter feed subscriptions.",
		h.mcpListFeeds)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_items",
		"Items ingested by any of the user's feeds, or with filtered_only "+
			"the ones filter rules hide. Article bodies are omitted — use "+
			"feeds_get_item for one item's content.",
		h.mcpListItems)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_get_item",
		"One feed item including its extracted article body.", h.mcpGetItem)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_filter_rules",
		"The user's rules hiding feed items by category or title, each with "+
			"how many items it filtered.", h.mcpListFilterRules)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_get_filter_rule_suggestions",
		"Feed categories the user rarely reads (at least 10 items in the last "+
			"90 days, at most 10% read), not yet covered by a rule or "+
			"dismissed, as candidate category filter rules.",
		h.mcpGetFilterRuleSuggestions)
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
		Limit:        args.Limit,
		Offset:       args.Offset,
		UnreadOnly:   proto.Bool(args.UnreadOnly),
		FilteredOnly: proto.Bool(args.FilteredOnly),
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

func (h *feedsConnectHandler) mcpListFilterRules(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.ListFilterRules(ctx, connect.NewRequest(
		&feedsv1.ListFilterRulesRequest{},
	)))
}

func (h *feedsConnectHandler) mcpGetFilterRuleSuggestions(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	return mcptools.Unwrap(h.GetFilterRuleSuggestions(ctx, connect.NewRequest(
		&feedsv1.GetFilterRuleSuggestionsRequest{},
	)))
}
