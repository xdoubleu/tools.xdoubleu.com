package trains

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	"tools.xdoubleu.com/apps/trains/internal/models"
	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
	"tools.xdoubleu.com/gen/trains/v1/trainsv1connect"
)

type trainsConnectHandler struct {
	app *Trains
}

var _ trainsv1connect.TrainServiceHandler = (*trainsConnectHandler)(nil)

func (h *trainsConnectHandler) SearchJourneys(
	ctx context.Context,
	req *connect.Request[trainsv1.SearchJourneysRequest],
) (*connect.Response[trainsv1.SearchJourneysResponse], error) {
	msg := req.Msg
	when := time.Now()
	if msg.GetTime() != "" {
		parsed, err := time.Parse(time.RFC3339, msg.GetTime())
		if err != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument, err,
			)
		}
		when = parsed
	}

	journeys, err := h.app.Services.Journey.SearchJourneys(
		ctx, msg.GetOriginStopId(), msg.GetDestinationStopId(), when, msg.GetArriveBy(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	out := make([]*trainsv1.Journey, len(journeys))
	for i, j := range journeys {
		out[i] = protoJourney(&j)
	}
	return connect.NewResponse(&trainsv1.SearchJourneysResponse{
		Journeys: out,
	}), nil
}

// GetJourneyDetail fetches the full live state of a previously-searched
// journey (issue #1394) and ensures the journey's websocket topic exists so
// a client can subscribe to live pushes right after this call returns.
func (h *trainsConnectHandler) GetJourneyDetail(
	ctx context.Context,
	req *connect.Request[trainsv1.GetJourneyDetailRequest],
) (*connect.Response[trainsv1.GetJourneyDetailResponse], error) {
	journeyID := req.Msg.GetJourneyId()
	if journeyID == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("journey_id is required"),
		)
	}

	detail, err := h.app.Services.JourneyDetail.GetJourneyDetail(ctx, journeyID)
	if err != nil {
		if errors.Is(err, services.ErrInvalidJourneyID) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	h.app.Services.JourneyWS.EnsureTopic(journeyID)

	return connect.NewResponse(&trainsv1.GetJourneyDetailResponse{
		Journey: protoJourneyDetail(journeyID, detail),
	}), nil
}

func protoJourney(j *csa.Journey) *trainsv1.Journey {
	legs := make([]*trainsv1.Leg, len(j.Legs))
	refs := make([]services.LegRef, len(j.Legs))
	for i, l := range j.Legs {
		legs[i] = &trainsv1.Leg{
			TripShortName:  l.TripShortName,
			RouteShortName: l.RouteShortName,
			Headsign:       l.Headsign,
			BoardStopId:    l.BoardStopID,
			BoardStopName:  l.BoardStopName,
			BoardPlatform:  l.BoardPlatform,
			BoardTime:      l.BoardTime.Format(time.RFC3339),
			AlightStopId:   l.AlightStopID,
			AlightStopName: l.AlightStopName,
			AlightPlatform: l.AlightPlatform,
			AlightTime:     l.AlightTime.Format(time.RFC3339),
		}
		refs[i] = services.LegRef{
			TripShortName: l.TripShortName,
			BoardStopID:   l.BoardStopID,
			BoardTime:     l.BoardTime,
			AlightStopID:  l.AlightStopID,
			AlightTime:    l.AlightTime,
		}
	}
	return &trainsv1.Journey{
		Legs:          legs,
		DepartureTime: j.DepartureTime.Format(time.RFC3339),
		ArrivalTime:   j.ArrivalTime.Format(time.RFC3339),
		Transfers:     int32(j.Transfers), //nolint:gosec //transfer count is small
		JourneyId:     services.EncodeJourneyID(refs),
	}
}

func protoJourneyDetail(
	journeyID string,
	d *models.JourneyDetail,
) *trainsv1.JourneyDetail {
	legs := make([]*trainsv1.LegDetail, len(d.Legs))
	for i := range d.Legs {
		legs[i] = protoLegDetail(&d.Legs[i])
	}
	return &trainsv1.JourneyDetail{
		JourneyId:     journeyID,
		Legs:          legs,
		DepartureTime: d.DepartureTime.Format(time.RFC3339),
		ArrivalTime:   d.ArrivalTime.Format(time.RFC3339),
		Alternative:   protoJourneyAlternative(d.Alternative),
	}
}

func protoJourneyAlternative(
	a *models.JourneyAlternative,
) *trainsv1.JourneyAlternative {
	if a == nil {
		return nil
	}
	return &trainsv1.JourneyAlternative{
		Reason:       a.Reason,
		FromStopName: a.FromStopName,
		Journey:      protoJourneyOption(a.Journey),
	}
}

func protoJourneyOption(o *models.JourneyOption) *trainsv1.Journey {
	if o == nil {
		return nil
	}
	legs := make([]*trainsv1.Leg, len(o.Legs))
	for i, l := range o.Legs {
		legs[i] = &trainsv1.Leg{
			TripShortName:  l.TripShortName,
			RouteShortName: l.RouteShortName,
			Headsign:       l.Headsign,
			BoardStopId:    l.BoardStopID,
			BoardStopName:  l.BoardStopName,
			BoardPlatform:  l.BoardPlatform,
			BoardTime:      l.BoardTime.Format(time.RFC3339),
			AlightStopId:   l.AlightStopID,
			AlightStopName: l.AlightStopName,
			AlightPlatform: l.AlightPlatform,
			AlightTime:     l.AlightTime.Format(time.RFC3339),
		}
	}
	return &trainsv1.Journey{
		Legs:          legs,
		DepartureTime: o.DepartureTime.Format(time.RFC3339),
		ArrivalTime:   o.ArrivalTime.Format(time.RFC3339),
		Transfers:     int32(o.Transfers), //nolint:gosec //transfer count is small
		JourneyId:     o.JourneyID,
	}
}

func protoLegDetail(l *models.LegDetail) *trainsv1.LegDetail {
	stops := make([]*trainsv1.StopCall, len(l.Stops))
	for i := range l.Stops {
		stops[i] = protoStopCall(&l.Stops[i])
	}
	alerts := make([]*trainsv1.Alert, len(l.Alerts))
	for i, a := range l.Alerts {
		alerts[i] = &trainsv1.Alert{
			Id:              a.ID,
			HeaderText:      a.HeaderText,
			DescriptionText: a.DescriptionText,
		}
	}
	return &trainsv1.LegDetail{
		TripShortName:  l.TripShortName,
		RouteShortName: l.RouteShortName,
		Headsign:       l.Headsign,
		Cancelled:      l.Cancelled,
		Stops:          stops,
		Alerts:         alerts,
	}
}

func protoStopCall(sc *models.StopDetail) *trainsv1.StopCall {
	out := &trainsv1.StopCall{
		StopId:       sc.StopID,
		StopName:     sc.StopName,
		Platform:     sc.Platform,
		Status:       sc.State.String(),
		IsBoardStop:  sc.IsBoardStop,
		IsAlightStop: sc.IsAlightStop,
	}
	if sc.ScheduledArrival != nil {
		out.ScheduledArrival = sc.ScheduledArrival.Format(time.RFC3339)
	}
	if sc.ScheduledDeparture != nil {
		out.ScheduledDeparture = sc.ScheduledDeparture.Format(time.RFC3339)
	}
	var delay int
	switch {
	case sc.ArrivalDelay != nil:
		delay = *sc.ArrivalDelay
	case sc.DepartureDelay != nil:
		delay = *sc.DepartureDelay
	}
	out.DelaySeconds = int32(delay)
	return out
}

func mapError(err error) error {
	switch {
	case errors.Is(err, csa.ErrUnknownStop):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, services.ErrRouterWarmingUp):
		return connect.NewError(connect.CodeUnavailable, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
