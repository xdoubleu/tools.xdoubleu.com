package models

import (
	"encoding/json"
	"time"
)

// StopDetail is one stop along a journey detail leg — static schedule data
// joined with (if any) the current realtime overlay for that call. State is
// never defaulted to DelayOnTime: no live reading means DelayUnknown, kept
// visually distinct from an on-time reading (issue #1394).
type StopDetail struct {
	StopID             string
	StopName           string
	Platform           string
	ScheduledArrival   *time.Time
	ScheduledDeparture *time.Time
	State              DelayState
	ArrivalDelay       *int
	DepartureDelay     *int
	IsBoardStop        bool
	IsAlightStop       bool
}

// stopDetailWire is the JSON shape StopDetail marshals to — deliberately the
// same shape trains.v1.StopCall's protobuf JSON encoding uses, so the
// journey websocket's pushed events (this package's own JSON DTO, following
// internal/progressws' convention of a plain wire DTO rather than
// protobuf-encoding a websocket payload) and the initial GetJourneyDetail
// RPC response decode into an identical client-side shape.
type stopDetailWire struct {
	StopID             string `json:"stopId"`
	StopName           string `json:"stopName"`
	Platform           string `json:"platform"`
	ScheduledArrival   string `json:"scheduledArrival,omitempty"`
	ScheduledDeparture string `json:"scheduledDeparture,omitempty"`
	Status             string `json:"status"`
	DelaySeconds       int    `json:"delaySeconds,omitempty"`
	IsBoardStop        bool   `json:"isBoardStop"`
	IsAlightStop       bool   `json:"isAlightStop"`
}

func (d StopDetail) MarshalJSON() ([]byte, error) {
	w := stopDetailWire{
		StopID:             d.StopID,
		StopName:           d.StopName,
		Platform:           d.Platform,
		ScheduledArrival:   "",
		ScheduledDeparture: "",
		Status:             d.State.String(),
		DelaySeconds:       0,
		IsBoardStop:        d.IsBoardStop,
		IsAlightStop:       d.IsAlightStop,
	}
	if d.ScheduledArrival != nil {
		w.ScheduledArrival = d.ScheduledArrival.Format(time.RFC3339)
	}
	if d.ScheduledDeparture != nil {
		w.ScheduledDeparture = d.ScheduledDeparture.Format(time.RFC3339)
	}
	switch {
	case d.ArrivalDelay != nil:
		w.DelaySeconds = *d.ArrivalDelay
	case d.DepartureDelay != nil:
		w.DelaySeconds = *d.DepartureDelay
	}
	return json.Marshal(w)
}

// LegDetail is one boarded train within a JourneyDetail, with its full
// stop-by-stop pattern between where the passenger boards and alights.
type LegDetail struct {
	TripShortName  string
	RouteShortName string
	Headsign       string
	// Cancelled is the whole-trip state — distinct from an individual
	// StopDetail's DelaySkipped (partial cancellation).
	Cancelled bool
	Stops     []StopDetail
	Alerts    []Alert
}

type legDetailWire struct {
	TripShortName  string       `json:"tripShortName"`
	RouteShortName string       `json:"routeShortName"`
	Headsign       string       `json:"headsign"`
	Cancelled      bool         `json:"cancelled"`
	Stops          []StopDetail `json:"stops"`
	Alerts         []Alert      `json:"alerts"`
}

func (l LegDetail) MarshalJSON() ([]byte, error) {
	stops := l.Stops
	if stops == nil {
		stops = []StopDetail{}
	}
	alerts := l.Alerts
	if alerts == nil {
		alerts = []Alert{}
	}
	return json.Marshal(legDetailWire{
		TripShortName:  l.TripShortName,
		RouteShortName: l.RouteShortName,
		Headsign:       l.Headsign,
		Cancelled:      l.Cancelled,
		Stops:          stops,
		Alerts:         alerts,
	})
}

// JourneyDetail is the full live state of one previously-searched journey
// (issue #1394) — every leg's stops, scheduled+live times, platform, and any
// attached service alerts.
type JourneyDetail struct {
	Legs          []LegDetail
	DepartureTime time.Time
	ArrivalTime   time.Time
	// Alternative is set only when a positive realtime signal shows the
	// planned journey no longer works (issue #1395) — nil in the common case.
	Alternative *JourneyAlternative
}

type journeyDetailWire struct {
	Legs          []LegDetail         `json:"legs"`
	DepartureTime string              `json:"departureTime"`
	ArrivalTime   string              `json:"arrivalTime"`
	Alternative   *JourneyAlternative `json:"alternative,omitempty"`
}

func (j JourneyDetail) MarshalJSON() ([]byte, error) {
	legs := j.Legs
	if legs == nil {
		legs = []LegDetail{}
	}
	return json.Marshal(journeyDetailWire{
		Legs:          legs,
		DepartureTime: j.DepartureTime.Format(time.RFC3339),
		ArrivalTime:   j.ArrivalTime.Format(time.RFC3339),
		Alternative:   j.Alternative,
	})
}

// JourneyAlternative is a re-planned itinerary surfaced when the open journey
// breaks (issue #1395). Reason explains what broke in passenger terms;
// Journey is the re-plan from FromStopName, nil when the router found none.
type JourneyAlternative struct {
	Reason       string
	FromStopName string
	Journey      *JourneyOption
}

type journeyAlternativeWire struct {
	Reason       string         `json:"reason"`
	FromStopName string         `json:"fromStopName"`
	Journey      *JourneyOption `json:"journey,omitempty"`
}

func (a JourneyAlternative) MarshalJSON() ([]byte, error) {
	return json.Marshal(journeyAlternativeWire(a))
}

// JourneyOption mirrors trains.v1.Journey — one planned itinerary, in the
// same JSON shape a SearchJourneys result serializes to, so the websocket
// push and the RPC response decode into an identical client-side type and
// the overview-row component renders it unchanged (issue #1395 / #1392).
type JourneyOption struct {
	Legs          []JourneyOptionLeg
	DepartureTime time.Time
	ArrivalTime   time.Time
	Transfers     int
	JourneyID     string
}

type journeyOptionWire struct {
	Legs          []JourneyOptionLeg `json:"legs"`
	DepartureTime string             `json:"departureTime"`
	ArrivalTime   string             `json:"arrivalTime"`
	Transfers     int                `json:"transfers"`
	JourneyID     string             `json:"journeyId"`
}

func (o JourneyOption) MarshalJSON() ([]byte, error) {
	legs := o.Legs
	if legs == nil {
		legs = []JourneyOptionLeg{}
	}
	return json.Marshal(journeyOptionWire{
		Legs:          legs,
		DepartureTime: o.DepartureTime.Format(time.RFC3339),
		ArrivalTime:   o.ArrivalTime.Format(time.RFC3339),
		Transfers:     o.Transfers,
		JourneyID:     o.JourneyID,
	})
}

// JourneyOptionLeg mirrors trains.v1.Leg.
type JourneyOptionLeg struct {
	TripShortName  string
	RouteShortName string
	Headsign       string
	BoardStopID    string
	BoardStopName  string
	BoardPlatform  string
	BoardTime      time.Time
	AlightStopID   string
	AlightStopName string
	AlightPlatform string
	AlightTime     time.Time
}

type journeyOptionLegWire struct {
	TripShortName  string `json:"tripShortName"`
	RouteShortName string `json:"routeShortName"`
	Headsign       string `json:"headsign"`
	BoardStopID    string `json:"boardStopId"`
	BoardStopName  string `json:"boardStopName"`
	BoardPlatform  string `json:"boardPlatform"`
	BoardTime      string `json:"boardTime"`
	AlightStopID   string `json:"alightStopId"`
	AlightStopName string `json:"alightStopName"`
	AlightPlatform string `json:"alightPlatform"`
	AlightTime     string `json:"alightTime"`
}

func (l JourneyOptionLeg) MarshalJSON() ([]byte, error) {
	return json.Marshal(journeyOptionLegWire{
		TripShortName:  l.TripShortName,
		RouteShortName: l.RouteShortName,
		Headsign:       l.Headsign,
		BoardStopID:    l.BoardStopID,
		BoardStopName:  l.BoardStopName,
		BoardPlatform:  l.BoardPlatform,
		BoardTime:      l.BoardTime.Format(time.RFC3339),
		AlightStopID:   l.AlightStopID,
		AlightStopName: l.AlightStopName,
		AlightPlatform: l.AlightPlatform,
		AlightTime:     l.AlightTime.Format(time.RFC3339),
	})
}
