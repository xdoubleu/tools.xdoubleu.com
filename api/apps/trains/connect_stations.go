package trains

import (
	"context"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

func (h *trainsConnectHandler) SearchStations(
	ctx context.Context,
	req *trainsv1.SearchStationsRequest,
) (*trainsv1.SearchStationsResponse, error) {
	stations, err := h.app.Services.Stations.SearchStations(ctx, req.GetQuery())
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*trainsv1.Station, len(stations))
	for i := range stations {
		out[i] = protoStation(&stations[i])
	}
	return &trainsv1.SearchStationsResponse{Stations: out}, nil
}
