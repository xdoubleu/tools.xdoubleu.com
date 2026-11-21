package connect

import (
	"context"

	"tools.xdoubleu.com/apps/movies/internal/protomap"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *Handler) GetStats(
	ctx context.Context,
	req *moviesv1.GetStatsRequest,
) (*moviesv1.GetStatsResponse, error) {
	uid, err := h.owner(ctx, req.Shared)
	if err != nil {
		return nil, err
	}

	stats, err := h.movies.Stats(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}

	return protomap.Stats(stats), nil
}
