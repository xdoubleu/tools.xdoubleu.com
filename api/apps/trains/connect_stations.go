package trains

import (
	"context"

	"connectrpc.com/connect"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

func (h *trainsConnectHandler) SearchStations(
	ctx context.Context,
	req *connect.Request[trainsv1.SearchStationsRequest],
) (*connect.Response[trainsv1.SearchStationsResponse], error) {
	stations, err := h.app.Services.Stations.SearchStations(ctx, req.Msg.GetQuery())
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*trainsv1.Station, len(stations))
	for i, st := range stations {
		out[i] = &trainsv1.Station{StopId: st.StopID, Name: st.Name}
	}
	return connect.NewResponse(&trainsv1.SearchStationsResponse{Stations: out}), nil
}

func (h *trainsConnectHandler) GetFeedInfo(
	ctx context.Context,
	_ *connect.Request[trainsv1.GetFeedInfoRequest],
) (*connect.Response[trainsv1.GetFeedInfoResponse], error) {
	version, err := h.app.Services.FeedInfo.FeedVersion(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	return connect.NewResponse(&trainsv1.GetFeedInfoResponse{FeedVersion: version}), nil
}
