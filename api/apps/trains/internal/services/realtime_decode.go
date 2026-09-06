package services

import (
	"fmt"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"google.golang.org/protobuf/proto"

	"tools.xdoubleu.com/apps/trains/internal/models"
)

// decodeTripUpdates parses a GTFS-RT trip-update FeedMessage into the
// domain model, keyed by raw trip_id (issue #1393).
func decodeTripUpdates(body []byte) (map[string]models.TripUpdate, error) {
	var msg gtfs.FeedMessage
	if err := proto.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("trains: decode trip-update feed: %w", err)
	}

	result := make(map[string]models.TripUpdate, len(msg.GetEntity()))
	for _, entity := range msg.GetEntity() {
		tu := entity.GetTripUpdate()
		if tu == nil {
			continue
		}

		trip := tu.GetTrip()
		tripID := trip.GetTripId()

		state := models.DelayOnTime
		if trip.GetScheduleRelationship() == gtfs.TripDescriptor_CANCELED {
			state = models.DelayCancelled
		}

		stopUpdates := tu.GetStopTimeUpdate()
		calls := make([]models.StopCall, 0, len(stopUpdates))
		for _, stu := range stopUpdates {
			calls = append(calls, decodeStopCall(stu))
		}

		result[tripID] = models.TripUpdate{
			TripID:    tripID,
			RouteID:   trip.GetRouteId(),
			StartDate: trip.GetStartDate(),
			State:     state,
			StopCalls: calls,
			Timestamp: unixSecondsOrZero(tu.Timestamp),
		}
	}
	return result, nil
}

// decodeStopCall applies the stop-level schedule_relationship table from
// issue #1393: SKIPPED is a partial cancellation, NO_DATA and UNSCHEDULED
// mean no live information, and only the SCHEDULED default carries a delay
// to interpret.
func decodeStopCall(stu *gtfs.TripUpdate_StopTimeUpdate) models.StopCall {
	call := models.StopCall{
		StopID:         stu.GetStopId(),
		StopSequence:   int(stu.GetStopSequence()),
		State:          models.DelayUnknown,
		ArrivalDelay:   nil,
		DepartureDelay: nil,
	}

	switch stu.GetScheduleRelationship() {
	case gtfs.TripUpdate_StopTimeUpdate_SKIPPED:
		call.State = models.DelaySkipped
		return call
	case gtfs.TripUpdate_StopTimeUpdate_NO_DATA,
		gtfs.TripUpdate_StopTimeUpdate_UNSCHEDULED:
		return call
	case gtfs.TripUpdate_StopTimeUpdate_SCHEDULED:
		// handled below
	}

	call.ArrivalDelay = stopEventDelay(stu.GetArrival())
	call.DepartureDelay = stopEventDelay(stu.GetDeparture())

	switch {
	case call.ArrivalDelay == nil && call.DepartureDelay == nil:
		call.State = models.DelayUnknown
	case (call.ArrivalDelay != nil && *call.ArrivalDelay != 0) ||
		(call.DepartureDelay != nil && *call.DepartureDelay != 0):
		call.State = models.DelayDelayed
	default:
		call.State = models.DelayOnTime
	}
	return call
}

func stopEventDelay(ev *gtfs.TripUpdate_StopTimeEvent) *int {
	if ev == nil || ev.Delay == nil {
		return nil
	}
	delay := int(ev.GetDelay())
	return &delay
}

// decodeAlerts parses a GTFS-RT alert FeedMessage into the domain model.
func decodeAlerts(body []byte) ([]models.Alert, error) {
	var msg gtfs.FeedMessage
	if err := proto.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("trains: decode alert feed: %w", err)
	}

	entities := msg.GetEntity()
	alerts := make([]models.Alert, 0, len(entities))
	for _, entity := range entities {
		a := entity.GetAlert()
		if a == nil {
			continue
		}

		alert := models.Alert{
			ID:               entity.GetId(),
			Cause:            a.GetCause().String(),
			Effect:           a.GetEffect().String(),
			HeaderText:       firstTranslation(a.GetHeaderText()),
			DescriptionText:  firstTranslation(a.GetDescriptionText()),
			InformedTripIDs:  nil,
			InformedRouteIDs: nil,
			InformedStopIDs:  nil,
		}
		for _, sel := range a.GetInformedEntity() {
			if tripID := sel.GetTrip().GetTripId(); tripID != "" {
				alert.InformedTripIDs = append(alert.InformedTripIDs, tripID)
			}
			if routeID := sel.GetRouteId(); routeID != "" {
				alert.InformedRouteIDs = append(alert.InformedRouteIDs, routeID)
			}
			if stopID := sel.GetStopId(); stopID != "" {
				alert.InformedStopIDs = append(alert.InformedStopIDs, stopID)
			}
		}
		alerts = append(alerts, alert)
	}
	return alerts, nil
}

func firstTranslation(ts *gtfs.TranslatedString) string {
	translations := ts.GetTranslation()
	if len(translations) == 0 {
		return ""
	}
	return translations[0].GetText()
}

func unixSecondsOrZero(ts *uint64) time.Time {
	if ts == nil {
		return time.Time{}
	}
	//nolint:gosec //feed timestamp, not attacker input
	return time.Unix(int64(*ts), 0).UTC()
}
