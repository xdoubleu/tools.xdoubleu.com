package feeds

import (
	"context"

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
	if err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.RestoreFeedItemResponse{
		Item: protoItem(*item),
	}, nil
}
