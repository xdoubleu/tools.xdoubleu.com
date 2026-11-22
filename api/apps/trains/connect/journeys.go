package connect

import (
	"context"

	"tools.xdoubleu.com/apps/trains/internal/protomap"
	"tools.xdoubleu.com/apps/trains/internal/services"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

func (h *Handler) SearchJourneys(
	ctx context.Context,
	req *trainsv1.SearchJourneysRequest,
) (*trainsv1.SearchJourneysResponse, error) {
	when, err := services.ParseSearchTime(req.GetTime())
	if err != nil {
		return nil, mapError(err)
	}

	journeys, err := h.svc.Journey.SearchJourneys(
		ctx, req.GetOriginStopId(), req.GetDestinationStopId(), when, req.GetArriveBy(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.SearchJourneysResponse{
		Journeys: protomap.Journeys(journeys),
	}, nil
}

// GetJourneyDetail fetches a searched journey's live state and ensures its
// websocket topic exists so the client can subscribe right after.
func (h *Handler) GetJourneyDetail(
	ctx context.Context,
	req *trainsv1.GetJourneyDetailRequest,
) (*trainsv1.GetJourneyDetailResponse, error) {
	journeyID := req.GetJourneyId()
	detail, err := h.svc.JourneyWatch.Watch(ctx, journeyID)
	if err != nil {
		return nil, mapError(err)
	}
	return &trainsv1.GetJourneyDetailResponse{
		Journey: protomap.JourneyDetail(journeyID, detail),
	}, nil
}
