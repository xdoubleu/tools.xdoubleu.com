package feeds

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/feeds/internal/services"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func (h *feedsConnectHandler) RestoreFeedItem(
	ctx context.Context,
	req *feedsv1.RestoreFeedItemRequest,
) (*feedsv1.RestoreFeedItemResponse, error) {
	user, cerr := feedUser(ctx)
	if cerr != nil {
		return nil, cerr
	}
	itemID, cerr := parseItemID(req.ItemId)
	if cerr != nil {
		return nil, cerr
	}

	item, err := h.app.Services.Feeds.RestoreItem(ctx, user.ID, itemID)
	if errors.Is(err, services.ErrItemNotFiltered) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err.Error()).
			WithCause(err)
	}
	if err != nil {
		return nil, feedErrorToConnect(err)
	}
	return &feedsv1.RestoreFeedItemResponse{
		Item: protoItem(*item),
	}, nil
}
