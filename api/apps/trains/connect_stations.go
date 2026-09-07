package trains

import (
	"context"
	"time"

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
		out[i] = &trainsv1.Station{
			StopId: st.StopID,
			NameNl: st.NameNL,
			NameFr: st.NameFR,
			NameEn: st.NameEN,
		}
	}
	return connect.NewResponse(&trainsv1.SearchStationsResponse{Stations: out}), nil
}

func (h *trainsConnectHandler) GetFeedInfo(
	ctx context.Context,
	_ *connect.Request[trainsv1.GetFeedInfoRequest],
) (*connect.Response[trainsv1.GetFeedInfoResponse], error) {
	info, err := h.app.Services.FeedInfo.FeedInfo(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	var importedAt string
	if info.ImportedAt != nil {
		importedAt = info.ImportedAt.Format(time.RFC3339)
	}
	return connect.NewResponse(&trainsv1.GetFeedInfoResponse{
		FeedVersion: info.FeedVersion,
		ImportedAt:  importedAt,
	}), nil
}
