package connect

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/apps/feeds/internal/protomap"
	"tools.xdoubleu.com/apps/feeds/internal/services"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/kit/database"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Handler serves feeds.v1.FeedService.
type Handler struct {
	feeds  *services.FeedService
	logger *slog.Logger
}

// NewHandler returns a Handler backed by feeds.
func NewHandler(feeds *services.FeedService, logger *slog.Logger) *Handler {
	return &Handler{feeds: feeds, logger: logger}
}

var _ feedsv1connect.FeedServiceHandler = (*Handler)(nil)

// feedKinds maps proto kinds to models.FeedSource*; UNSPECIFIED maps to ""
// and creates an RSS/Atom feed.
//
//nolint:gochecknoglobals // static lookup table, read-only after init
var feedKinds = map[feedsv1.FeedKind]string{
	feedsv1.FeedKind_FEED_KIND_UNSPECIFIED: "",
	feedsv1.FeedKind_FEED_KIND_RSS:         models.FeedSourceRSS,
	feedsv1.FeedKind_FEED_KIND_EMAIL:       models.FeedSourceEmail,
	feedsv1.FeedKind_FEED_KIND_SCRAPE:      models.FeedSourceScrape,
}

// parseItemID mirrors parseFeedID for the item-scoped RPCs.
func parseItemID(id string) (uuid.UUID, *connect.Error) {
	itemID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid item ID",
		)
	}
	return itemID, nil
}

// feedUser resolves the authenticated user for feed RPCs.
func feedUser(ctx context.Context) (*sharedmodels.User, *connect.Error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			"unauthorized",
		)
	}
	return user, nil
}

func parseFeedID(id string) (uuid.UUID, *connect.Error) {
	feedID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument,
			"invalid feed ID",
		)
	}
	return feedID, nil
}

func mapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.Code(services.ErrInvalidFeed, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrUnsupportedURL, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrNoPostsFound, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidFilterRule, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrInvalidExpiryDays, connect.CodeInvalidArgument),
		connecttools.Code(
			services.ErrEmailFeedsNotConfigured, connect.CodeFailedPrecondition,
		),
		connecttools.Code(services.ErrItemNotFiltered, connect.CodeFailedPrecondition),
		connecttools.Code(services.ErrEmailFeedHasURL, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrAdminRequired, connect.CodePermissionDenied),
	)
}

func (h *Handler) ListFeeds(
	ctx context.Context,
	_ *feedsv1.ListFeedsRequest,
) (*feedsv1.ListFeedsResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	feeds, err := h.feeds.List(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*feedsv1.Feed, len(feeds))
	for i, f := range feeds {
		out[i] = protomap.Feed(f)
	}
	return &feedsv1.ListFeedsResponse{Feeds: out}, nil
}

func (h *Handler) CreateFeed(
	ctx context.Context,
	req *feedsv1.CreateFeedRequest,
) (*feedsv1.CreateFeedResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	feed, address, err := h.feeds.CreateFeed(ctx, user.ID, services.NewFeed{
		Kind:  feedKinds[req.Kind],
		URL:   req.Url,
		Title: req.Title,
	})
	if err != nil {
		return nil, mapError(err)
	}

	proto := protomap.Feed(*feed)
	proto.InboundAddress = address
	return &feedsv1.CreateFeedResponse{Feed: proto}, nil
}

func (h *Handler) UpdateFeed(
	ctx context.Context,
	req *feedsv1.UpdateFeedRequest,
) (*feedsv1.UpdateFeedResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	var expiryDays *int
	if req.ExpiryDays != nil {
		days := int(req.GetExpiryDays())
		expiryDays = &days
	}
	if err := h.feeds.Update(
		ctx, user.ID, feedID, req.Title, expiryDays,
	); err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.UpdateFeedResponse{}, nil
}

func (h *Handler) DeleteFeed(
	ctx context.Context,
	req *feedsv1.DeleteFeedRequest,
) (*feedsv1.DeleteFeedResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	if err := h.feeds.Delete(ctx, user.ID, feedID); err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.DeleteFeedResponse{}, nil
}

func (h *Handler) RefreshFeed(
	ctx context.Context,
	req *feedsv1.RefreshFeedRequest,
) (*feedsv1.RefreshFeedResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	ingested, err := h.feeds.Refresh(ctx, user.ID, feedID)
	if err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.RefreshFeedResponse{
		Ingested: int32(ingested), //nolint:gosec // bounded by per-poll cap
	}, nil
}

func (h *Handler) ListFeedItems(
	ctx context.Context,
	req *feedsv1.ListFeedItemsRequest,
) (*feedsv1.ListFeedItemsResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	var feedID *uuid.UUID
	if req.FeedId != nil {
		parsed, perr := parseFeedID(req.GetFeedId())
		if perr != nil {
			return nil, perr
		}
		feedID = &parsed
	}

	items, hasMore, err := h.feeds.ListItems(
		ctx, user.ID, req.Limit, req.Offset, req.GetUnreadOnly(),
		feedID, req.GetBookmarkedOnly(), listView(req),
	)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*feedsv1.Item, len(items))
	for i, item := range items {
		out[i] = protomap.Item(item)
	}
	return &feedsv1.ListFeedItemsResponse{
		Items:   out,
		HasMore: hasMore,
	}, nil
}

// GetFeedItem is the only RPC returning an item's article body.
func (h *Handler) GetFeedItem(
	ctx context.Context,
	req *feedsv1.GetFeedItemRequest,
) (*feedsv1.GetFeedItemResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	itemID, cerr := parseItemID(req.ItemId)
	if cerr != nil {
		return nil, cerr
	}

	item, err := h.feeds.GetItem(ctx, user.ID, itemID)
	if err != nil {
		if errors.Is(err, database.ErrResourceNotFound) {
			// A cached list can reference a deleted item; not_found emits no
			// Sentry event, so log it.
			h.logger.WarnContext(ctx, "feed item not found",
				"item_id", itemID, "user_id", user.ID)
		}
		return nil, mapError(err)
	}
	return &feedsv1.GetFeedItemResponse{
		Item: protomap.Item(*item),
	}, nil
}

func (h *Handler) UpdateItem(
	ctx context.Context,
	req *feedsv1.UpdateItemRequest,
) (*feedsv1.UpdateItemResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	itemID, cerr := parseItemID(req.ItemId)
	if cerr != nil {
		return nil, cerr
	}

	item, err := h.feeds.UpdateItem(
		ctx, user.ID, itemID, req.Read, req.Dismissed, req.Bookmarked,
		req.ReadProgressPct,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.UpdateItemResponse{
		Item: protomap.Item(*item),
	}, nil
}

func (h *Handler) GetFeedStats(
	ctx context.Context,
	_ *feedsv1.GetFeedStatsRequest,
) (*feedsv1.GetFeedStatsResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	stats, perDay, err := h.feeds.GetStats(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	protoStats := make([]*feedsv1.FeedStats, len(stats))
	for i, s := range stats {
		protoStats[i] = &feedsv1.FeedStats{
			FeedId:             s.FeedID.String(),
			FeedTitle:          s.FeedTitle,
			ItemCount:          int32(s.ItemCount), //nolint:gosec // small counts
			AvgIntervalHours:   s.AvgIntervalHours,
			ReadRate:           s.ReadRate,
			AvgReadProgressPct: s.AvgReadProgressPct,
		}
	}
	protoDays := make([]*feedsv1.DayCount, len(perDay))
	for i, d := range perDay {
		protoDays[i] = &feedsv1.DayCount{
			Day:   d.Day.Format(time.RFC3339),
			Count: int32(d.Count), //nolint:gosec // small counts
		}
	}

	return &feedsv1.GetFeedStatsResponse{
		Stats:       protoStats,
		ItemsPerDay: protoDays,
	}, nil
}

func (h *Handler) GetUnhealthyFeeds(
	ctx context.Context,
	_ *feedsv1.GetUnhealthyFeedsRequest,
) (*feedsv1.GetUnhealthyFeedsResponse, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	unhealthy, err := h.feeds.ListUnhealthyAs(ctx, user)
	if err != nil {
		return nil, mapError(err)
	}

	protoFeeds := make([]*feedsv1.UnhealthyFeed, len(unhealthy))
	for i, feed := range unhealthy {
		lastError := ""
		if feed.LastError != nil {
			lastError = *feed.LastError
		}
		protoFeeds[i] = &feedsv1.UnhealthyFeed{
			Title:     feed.Title,
			Url:       feed.URL,
			LastError: lastError,
			//nolint:gosec // failure counts fit int32
			ConsecutiveFailures: int32(feed.ConsecutiveFailures),
		}
	}
	return &feedsv1.GetUnhealthyFeedsResponse{Feeds: protoFeeds}, nil
}

// listView maps the request's view flags to a service view; last chance wins
// over expired, which wins over filtered.
func listView(req *feedsv1.ListFeedItemsRequest) services.ListView {
	switch {
	case req.GetLastChanceOnly():
		return services.ViewLastChance
	case req.GetExpiredOnly():
		return services.ViewExpired
	case req.GetFilteredOnly():
		return services.ViewFiltered
	default:
		return services.ViewInbox
	}
}

// ListTopFeedItems ranks today's best items from the caller's history.
func (h *Handler) ListTopFeedItems(
	ctx context.Context,
	_ *feedsv1.ListTopFeedItemsRequest,
) (*feedsv1.ListTopFeedItemsResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	items, err := h.feeds.ListTopItems(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}
	out := make([]*feedsv1.Item, len(items))
	for i, item := range items {
		out[i] = protomap.Item(item)
	}
	return &feedsv1.ListTopFeedItemsResponse{Items: out}, nil
}
