// Package mcp exposes the feeds app's read RPCs as MCP tools.
package mcp

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/feeds/internal/protomap"
	"tools.xdoubleu.com/apps/feeds/internal/services"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/mcptools"
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

// Register exposes the app's read-only RPCs as MCP tools, scoped to the
// caller.
func Register(srv *mcp.Server, feeds *services.FeedService, logger *slog.Logger) {
	t := tools{feeds: feeds, logger: logger}

	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_feeds",
		"The user's RSS/Atom and email-newsletter feed subscriptions.",
		t.mcpListFeeds)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_items",
		"Items ingested by any of the user's feeds, or with filtered_only "+
			"the ones filter rules hide. Article bodies are omitted — use "+
			"feeds_get_item for one item's content.",
		t.mcpListItems)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_get_item",
		"One feed item including its extracted article body.", t.mcpGetItem)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_list_filter_rules",
		"The user's rules hiding feed items by category or title, each with "+
			"how many items it filtered.", t.mcpListFilterRules)
	mcptools.AddReadTool(srv, mcpAppName, "feeds_get_filter_rule_suggestions",
		"Feed categories the user rarely reads (at least 10 items in the last "+
			"90 days, at most 10% read), not yet covered by a rule or "+
			"dismissed, as candidate category filter rules.",
		t.mcpGetFilterRuleSuggestions)
}

type tools struct {
	feeds  *services.FeedService
	logger *slog.Logger
}

type mcpGetItemArgs struct {
	ItemID string `json:"item_id" jsonschema:"the item's id"`
}

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(connect.CodeUnauthenticated, "unauthorized")
	}
	return user.ID, nil
}

func parseID(raw, what string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument, "invalid "+what+" ID",
		)
	}
	return id, nil
}

func (t tools) mcpListFeeds(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	feeds, err := t.feeds.List(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*feedsv1.Feed, len(feeds))
	for i, f := range feeds {
		out[i] = protomap.Feed(f)
	}
	return &feedsv1.ListFeedsResponse{Feeds: out}, nil
}

func (t tools) mcpListItems(
	ctx context.Context, args mcpListItemsArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	var feedID *uuid.UUID
	if args.FeedID != "" {
		parsed, perr := parseID(args.FeedID, "feed")
		if perr != nil {
			return nil, perr
		}
		feedID = &parsed
	}
	view := services.ViewInbox
	if args.FilteredOnly {
		view = services.ViewFiltered
	}
	items, hasMore, err := t.feeds.ListItems(
		ctx, uid, args.Limit, args.Offset, args.UnreadOnly, feedID, false, view,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*feedsv1.Item, len(items))
	for i, item := range items {
		out[i] = protomap.Item(item)
	}
	return &feedsv1.ListFeedItemsResponse{Items: out, HasMore: hasMore}, nil
}

func (t tools) mcpGetItem(
	ctx context.Context, args mcpGetItemArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	itemID, err := parseID(args.ItemID, "item")
	if err != nil {
		return nil, err
	}
	item, err := t.feeds.GetItem(ctx, uid, itemID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			t.logger.WarnContext(ctx, "feed item not found",
				"item_id", itemID, "user_id", uid)
		}
		return nil, connecttools.MapError(err)
	}
	return &feedsv1.GetFeedItemResponse{Item: protomap.Item(*item)}, nil
}

func (t tools) mcpListFilterRules(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := t.feeds.ListFilterRules(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*feedsv1.FilterRule, len(rules))
	for i, r := range rules {
		out[i] = protomap.FilterRule(r)
	}
	return &feedsv1.ListFilterRulesResponse{Rules: out}, nil
}

func (t tools) mcpGetFilterRuleSuggestions(
	ctx context.Context, _ mcptools.NoArgs,
) (proto.Message, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}
	suggestions, err := t.feeds.GetFilterRuleSuggestions(ctx, uid)
	if err != nil {
		return nil, connecttools.MapError(err)
	}
	out := make([]*feedsv1.FilterRuleSuggestion, len(suggestions))
	for i, sg := range suggestions {
		out[i] = protomap.FilterRuleSuggestion(sg)
	}
	return &feedsv1.GetFilterRuleSuggestionsResponse{Suggestions: out}, nil
}
