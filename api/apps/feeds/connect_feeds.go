package feeds

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	"tools.xdoubleu.com/apps/feeds/internal/services"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
	"tools.xdoubleu.com/gen/feeds/v1/feedsv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// feedsConnectHandler serves feeds.v1.FeedService.
type feedsConnectHandler struct {
	app *Feeds
}

var _ feedsv1connect.FeedServiceHandler = (*feedsConnectHandler)(nil)

func protoFeed(f models.Feed) *feedsv1.Feed {
	lastFetched := ""
	if f.LastFetchedAt != nil {
		lastFetched = f.LastFetchedAt.Format(time.RFC3339)
	}
	lastError := ""
	if f.LastError != nil {
		lastError = *f.LastError
	}
	return &feedsv1.Feed{
		Id:            f.ID.String(),
		Url:           f.URL,
		Title:         f.Title,
		LastFetchedAt: lastFetched,
		LastError:     lastError,
		CreatedAt:     f.CreatedAt.Format(time.RFC3339),
		SourceType:    f.SourceType,
	}
}

func protoItem(item models.Item) *feedsv1.Item {
	readAt := ""
	if item.ReadAt != nil {
		readAt = item.ReadAt.Format(time.RFC3339)
	}
	ingestError := ""
	if item.IngestError != nil {
		ingestError = *item.IngestError
	}
	publishedAt := ""
	if !item.PublishedAt.IsZero() {
		publishedAt = item.PublishedAt.Format(time.RFC3339)
	}
	return &feedsv1.Item{
		Id:          item.ID.String(),
		FeedId:      item.FeedID.String(),
		Title:       item.Title,
		SourceUrl:   item.SourceURL,
		ContentHtml: item.ContentHTML,
		PublishedAt: publishedAt,
		ReadAt:      readAt,
		Dismissed:   item.Dismissed,
		Favourite:   item.Favourite,
		IngestError: ingestError,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
	}
}

// feedUser resolves the authenticated user for feed RPCs.
func feedUser(ctx context.Context) (*sharedmodels.User, *connect.Error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			errors.New("unauthorized"),
		)
	}
	return user, nil
}

func parseFeedID(id string) (uuid.UUID, *connect.Error) {
	feedID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid feed ID"),
		)
	}
	return feedID, nil
}

func feedErrorToConnect(err error) *connect.Error {
	switch {
	case errors.Is(err, database.ErrResourceNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, database.ErrResourceConflict):
		return connect.NewError(
			connect.CodeAlreadyExists,
			errors.New("feed already exists"),
		)
	case errors.Is(err, services.ErrInvalidFeed),
		errors.Is(err, services.ErrUnsupportedURL):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func (h *feedsConnectHandler) ListFeeds(
	ctx context.Context,
	_ *connect.Request[feedsv1.ListFeedsRequest],
) (*connect.Response[feedsv1.ListFeedsResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	feeds, err := h.app.Services.Feeds.List(ctx, user.ID)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}

	out := make([]*feedsv1.Feed, len(feeds))
	for i, f := range feeds {
		out[i] = protoFeed(f)
	}
	return connect.NewResponse(&feedsv1.ListFeedsResponse{Feeds: out}), nil
}

func (h *feedsConnectHandler) CreateFeed(
	ctx context.Context,
	req *connect.Request[feedsv1.CreateFeedRequest],
) (*connect.Response[feedsv1.CreateFeedResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	if req.Msg.Kind == feedsv1.FeedKind_FEED_KIND_EMAIL {
		return h.createEmailFeed(ctx, user.ID, req.Msg)
	}

	feed, err := h.app.Services.Feeds.Create(ctx, user.ID, req.Msg.Url)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}

	return connect.NewResponse(&feedsv1.CreateFeedResponse{
		Feed: protoFeed(*feed),
	}), nil
}

func (h *feedsConnectHandler) createEmailFeed(
	ctx context.Context,
	userID string,
	req *feedsv1.CreateFeedRequest,
) (*connect.Response[feedsv1.CreateFeedResponse], error) {
	if req.Url != "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("url must be empty for an email feed"),
		)
	}

	feed, address, err := h.app.Services.Feeds.CreateEmail(ctx, userID, req.Title)
	if err != nil {
		if errors.Is(err, services.ErrEmailFeedsNotConfigured) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, feedErrorToConnect(err)
	}

	proto := protoFeed(*feed)
	proto.InboundAddress = address
	return connect.NewResponse(&feedsv1.CreateFeedResponse{Feed: proto}), nil
}

func (h *feedsConnectHandler) UpdateFeed(
	ctx context.Context,
	req *connect.Request[feedsv1.UpdateFeedRequest],
) (*connect.Response[feedsv1.UpdateFeedResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.Msg.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	if err := h.app.Services.Feeds.Update(
		ctx, user.ID, feedID, req.Msg.Title,
	); err != nil {
		return nil, feedErrorToConnect(err)
	}
	return connect.NewResponse(&feedsv1.UpdateFeedResponse{}), nil
}

func (h *feedsConnectHandler) DeleteFeed(
	ctx context.Context,
	req *connect.Request[feedsv1.DeleteFeedRequest],
) (*connect.Response[feedsv1.DeleteFeedResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.Msg.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	if err := h.app.Services.Feeds.Delete(ctx, user.ID, feedID); err != nil {
		return nil, feedErrorToConnect(err)
	}
	return connect.NewResponse(&feedsv1.DeleteFeedResponse{}), nil
}

func (h *feedsConnectHandler) RefreshFeed(
	ctx context.Context,
	req *connect.Request[feedsv1.RefreshFeedRequest],
) (*connect.Response[feedsv1.RefreshFeedResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.Msg.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	ingested, err := h.app.Services.Feeds.Refresh(ctx, user.ID, feedID)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}
	return connect.NewResponse(&feedsv1.RefreshFeedResponse{
		Ingested: int32(ingested), //nolint:gosec // bounded by per-poll cap
	}), nil
}

func (h *feedsConnectHandler) ListFeedItems(
	ctx context.Context,
	_ *connect.Request[feedsv1.ListFeedItemsRequest],
) (*connect.Response[feedsv1.ListFeedItemsResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	items, err := h.app.Services.Feeds.ListItems(ctx, user.ID)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}

	out := make([]*feedsv1.Item, len(items))
	for i, item := range items {
		out[i] = protoItem(item)
	}
	return connect.NewResponse(&feedsv1.ListFeedItemsResponse{Items: out}), nil
}

func (h *feedsConnectHandler) UpdateItem(
	ctx context.Context,
	req *connect.Request[feedsv1.UpdateItemRequest],
) (*connect.Response[feedsv1.UpdateItemResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	itemID, err := uuid.Parse(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			errors.New("invalid item ID"),
		)
	}

	item, err := h.app.Services.Feeds.UpdateItem(
		ctx, user.ID, itemID, req.Msg.Read, req.Msg.Dismissed, req.Msg.Favourite,
	)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}
	return connect.NewResponse(&feedsv1.UpdateItemResponse{
		Item: protoItem(*item),
	}), nil
}
