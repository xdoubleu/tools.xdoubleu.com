package trains

import (
	"context"
	"time"

	"connectrpc.com/connect"

	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

// GetRealtimeTrip exposes the realtime snapshot for one train running today.
func (h *trainsConnectHandler) GetRealtimeTrip(
	ctx context.Context,
	req *connect.Request[trainsv1.GetRealtimeTripRequest],
) (*connect.Response[trainsv1.GetRealtimeTripResponse], error) {
	view, err := h.app.Services.JourneyDetail.RealtimeTrip(
		ctx, req.Msg.GetTripShortName(), time.Now(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	snap := view.Snapshot
	var fetchedAt string
	if !snap.FetchedAt.IsZero() {
		fetchedAt = snap.FetchedAt.Format(time.RFC3339)
	}
	states := make(map[string]int32, len(view.StopCallStates))
	for state, n := range view.StopCallStates {
		states[state] = count32(n)
	}

	updates := make([]*trainsv1.RealtimeStopUpdate, len(view.Trip.StopCalls))
	for i, c := range view.Trip.StopCalls {
		updates[i] = &trainsv1.RealtimeStopUpdate{
			StopId:         c.StopID,
			StopSequence:   count32(c.StopSequence),
			Status:         c.State.String(),
			ArrivalDelay:   delay32(c.ArrivalDelay),
			DepartureDelay: delay32(c.DepartureDelay),
			ArrivalTime:    rfc3339OrEmpty(c.ArrivalTime),
			DepartureTime:  rfc3339OrEmpty(c.DepartureTime),
		}
	}
	planned := make([]*trainsv1.PlannedStop, len(view.PlannedStops))
	for i, st := range view.PlannedStops {
		planned[i] = &trainsv1.PlannedStop{
			StopId:       st.StopID,
			StopSequence: count32(st.StopSequence),
		}
	}

	var tripStatus string
	if view.Found {
		tripStatus = view.Trip.State.String()
	}
	return connect.NewResponse(&trainsv1.GetRealtimeTripResponse{
		FetchedAt:           fetchedAt,
		TripCount:           count32(len(snap.Trips)),
		UnresolvedTripCount: count32(snap.UnresolvedTripCount),
		DuplicateTripCount:  count32(snap.DuplicateTripCount),
		AlertCount:          count32(len(snap.Alerts)),
		StopUpdateStatuses:  states,
		Found:               view.Found,
		TripId:              view.Trip.TripID,
		StartDate:           view.Trip.StartDate,
		Status:              tripStatus,
		StopUpdates:         updates,
		PlannedTripId:       view.PlannedTripID,
		PlannedStops:        planned,
	}), nil
}

// delay32 keeps an unpublished delay absent rather than zero.
func delay32(d *int) *int32 {
	if d == nil {
		return nil
	}
	v := int32(*d) //nolint:gosec //a delay in seconds is far below int32 range
	return &v
}

func rfc3339OrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
