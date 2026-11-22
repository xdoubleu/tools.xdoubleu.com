// Package protomap maps the trains domain models to their wire messages, for
// the Connect and MCP adapters.
package protomap

import (
	"time"

	"tools.xdoubleu.com/apps/trains/internal/models"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

// Station maps a station.
func Station(st *models.Station) *trainsv1.Station {
	return &trainsv1.Station{
		StopId:      st.StopID,
		NameNl:      st.NameNL,
		NameFr:      st.NameFR,
		NameEn:      st.NameEN,
		DisplayName: st.DisplayName,
	}
}

// Stations maps a station list.
func Stations(list []models.Station) []*trainsv1.Station {
	out := make([]*trainsv1.Station, len(list))
	for i := range list {
		out[i] = Station(&list[i])
	}
	return out
}

// Journeys maps searched journey options.
func Journeys(list []models.JourneyOption) []*trainsv1.Journey {
	out := make([]*trainsv1.Journey, len(list))
	for i := range list {
		out[i] = Journey(&list[i])
	}
	return out
}

// JourneyDetail maps a journey's live state.
func JourneyDetail(
	journeyID string,
	d *models.JourneyDetail,
) *trainsv1.JourneyDetail {
	legs := make([]*trainsv1.LegDetail, len(d.Legs))
	for i := range d.Legs {
		legs[i] = legDetail(&d.Legs[i])
	}
	return &trainsv1.JourneyDetail{
		JourneyId:     journeyID,
		Legs:          legs,
		DepartureTime: d.DepartureTime.Format(time.RFC3339),
		ArrivalTime:   d.ArrivalTime.Format(time.RFC3339),
		Alternative:   journeyAlternative(d.Alternative),
	}
}

func journeyAlternative(
	a *models.JourneyAlternative,
) *trainsv1.JourneyAlternative {
	if a == nil {
		return nil
	}
	return &trainsv1.JourneyAlternative{
		Reason:       a.Reason,
		FromStopName: a.FromStopName,
		Journey:      Journey(a.Journey),
	}
}

// Journey maps a searched journey option; nil stays nil.
func Journey(o *models.JourneyOption) *trainsv1.Journey {
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

func legDetail(l *models.LegDetail) *trainsv1.LegDetail {
	stops := make([]*trainsv1.StopCall, len(l.Stops))
	for i := range l.Stops {
		stops[i] = stopCall(&l.Stops[i])
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

func stopCall(sc *models.StopDetail) *trainsv1.StopCall {
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
