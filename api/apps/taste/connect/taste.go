package connect

import (
	"context"

	"tools.xdoubleu.com/apps/internal/consumption"
	"tools.xdoubleu.com/apps/taste/internal/protomap"
	tastev1 "tools.xdoubleu.com/gen/taste/v1"
)

func (h *Handler) GetStats(
	ctx context.Context,
	req *tastev1.GetStatsRequest,
) (*tastev1.GetStatsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	stats, rates, err := h.svc.Reads.Stats(ctx, sc)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.Stats(stats, rates), nil
}

func (h *Handler) ListEvents(
	ctx context.Context,
	req *tastev1.ListEventsRequest,
) (*tastev1.ListEventsResponse, error) {
	sc, err := h.scopeOf(ctx, req.Shared)
	if err != nil {
		return nil, err
	}
	page, hasMore, err := h.svc.Reads.Events(
		ctx, sc, consumption.Domain(req.Domain), req.Limit, req.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}
	return protomap.Events(page, hasMore), nil
}
