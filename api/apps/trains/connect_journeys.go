package trains

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

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

func protoJourney(j *csa.Journey) *trainsv1.Journey {
	legs := make([]*trainsv1.Leg, len(j.Legs))
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
	}
	return &trainsv1.Journey{
		Legs:          legs,
		DepartureTime: j.DepartureTime.Format(time.RFC3339),
		ArrivalTime:   j.ArrivalTime.Format(time.RFC3339),
		Transfers:     int32(j.Transfers), //nolint:gosec //transfer count is small
	}
}

func mapError(err error) error {
	if errors.Is(err, csa.ErrUnknownStop) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
