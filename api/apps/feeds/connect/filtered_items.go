package connect

import (
	"context"

	"tools.xdoubleu.com/apps/feeds/internal/protomap"
	feedsv1 "tools.xdoubleu.com/gen/feeds/v1"
)

func (h *Handler) RestoreFeedItem(
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

	item, err := h.feeds.RestoreItem(ctx, user.ID, itemID)
	if err != nil {
		return nil, mapError(err)
	}
	return &feedsv1.RestoreFeedItemResponse{
		Item: protomap.Item(*item),
	}, nil
}
