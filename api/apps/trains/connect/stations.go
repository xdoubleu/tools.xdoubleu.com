package connect

import (
	"context"

	"tools.xdoubleu.com/apps/trains/internal/protomap"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

func (h *Handler) SearchStations(
	ctx context.Context,
	req *trainsv1.SearchStationsRequest,
) (*trainsv1.SearchStationsResponse, error) {
	stations, err := h.svc.Stations.SearchStations(ctx, req.GetQuery())
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.SearchStationsResponse{
		Stations: protomap.Stations(stations),
	}, nil
}
