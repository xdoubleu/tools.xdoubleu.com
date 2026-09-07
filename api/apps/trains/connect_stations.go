package trains

import (
	"context"
	"math"
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
		Translations: &trainsv1.TranslationCoverage{
			TranslatedStopsNl: count32(info.Translations.StopsNL),
			TranslatedStopsFr: count32(info.Translations.StopsFR),
			TranslatedStopsEn: count32(info.Translations.StopsEN),
			Rows:              count32(info.Translations.Rows),
			RowsUnmatched:     count32(info.Translations.RowsUnmatched),
		},
	}), nil
}

// count32 narrows a parsed count to the proto's int32. A GTFS feed nowhere
// near two billion translation rows makes the clamp unreachable in practice;
// it is here so the conversion cannot wrap into a negative count.
func count32(n int) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	if n < 0 {
		return 0
	}
	return int32(n)
}
