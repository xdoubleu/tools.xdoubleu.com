package reading

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/reading/internal/models"
	"tools.xdoubleu.com/apps/reading/internal/services"
	readingv1 "tools.xdoubleu.com/gen/reading/v1"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func protoFeed(f models.Feed) *readingv1.Feed {
	lastFetched := ""
	if f.LastFetchedAt != nil {
		lastFetched = f.LastFetchedAt.Format(time.RFC3339)
	}
	lastError := ""
	if f.LastError != nil {
		lastError = *f.LastError
	}
	return &readingv1.Feed{
		Id:            f.ID.String(),
		Url:           f.URL,
		Title:         f.Title,
		KoboSync:      f.KoboSync,
		LastFetchedAt: lastFetched,
		LastError:     lastError,
		CreatedAt:     f.CreatedAt.Format(time.RFC3339),
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

func (h *booksConnectHandler) ListFeeds(
	ctx context.Context,
	_ *connect.Request[readingv1.ListFeedsRequest],
) (*connect.Response[readingv1.ListFeedsResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	feeds, err := h.app.Services.Feeds.List(ctx, user.ID)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}

	out := make([]*readingv1.Feed, len(feeds))
	for i, f := range feeds {
		out[i] = protoFeed(f)
	}
	return connect.NewResponse(&readingv1.ListFeedsResponse{Feeds: out}), nil
}

func (h *booksConnectHandler) CreateFeed(
	ctx context.Context,
	req *connect.Request[readingv1.CreateFeedRequest],
) (*connect.Response[readingv1.CreateFeedResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}

	feed, ingested, err := h.app.Services.Feeds.Create(
		ctx, user.ID, req.Msg.Url, req.Msg.KoboSync,
	)
	if err != nil {
		return nil, feedErrorToConnect(err)
	}

	return connect.NewResponse(&readingv1.CreateFeedResponse{
		Feed:     protoFeed(*feed),
		Ingested: int32(ingested), //nolint:gosec // bounded by per-poll cap
	}), nil
}

func (h *booksConnectHandler) UpdateFeed(
	ctx context.Context,
	req *connect.Request[readingv1.UpdateFeedRequest],
) (*connect.Response[readingv1.UpdateFeedResponse], error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	feedID, cerr := parseFeedID(req.Msg.FeedId)
	if cerr != nil {
		return nil, cerr
	}

	if err := h.app.Services.Feeds.Update(
		ctx, user.ID, feedID, req.Msg.Title, req.Msg.KoboSync,
	); err != nil {
		return nil, feedErrorToConnect(err)
	}
	return connect.NewResponse(&readingv1.UpdateFeedResponse{}), nil
}

func (h *booksConnectHandler) DeleteFeed(
	ctx context.Context,
	req *connect.Request[readingv1.DeleteFeedRequest],
) (*connect.Response[readingv1.DeleteFeedResponse], error) {
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
	return connect.NewResponse(&readingv1.DeleteFeedResponse{}), nil
}

func (h *booksConnectHandler) RefreshFeed(
	ctx context.Context,
	req *connect.Request[readingv1.RefreshFeedRequest],
) (*connect.Response[readingv1.RefreshFeedResponse], error) {
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
	return connect.NewResponse(&readingv1.RefreshFeedResponse{
		Ingested: int32(ingested), //nolint:gosec // bounded by per-poll cap
	}), nil
}
