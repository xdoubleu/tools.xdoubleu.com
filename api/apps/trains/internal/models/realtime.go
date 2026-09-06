package models

import "time"

// DelayState is the domain-level state of a single call, kept distinct from
// GTFS-RT's raw schedule_relationship values so no downstream caller can
// collapse them by accident (issue #1393). schedule_relationship means
// different things at trip level (CANCELED) and stop level (SKIPPED,
// NO_DATA) — conflating a stop-level NO_DATA with a cancellation once
// produced 12,703 false "cancelled" calls against a true network-wide
// figure of 48.
type DelayState int

const (
	// DelayUnknown means no live information was published for this call —
	// the common case (68% of calls in one observed sample), not the edge
	// case. It is never equivalent to DelayOnTime: a zero-valued delay field
	// would otherwise silently mean both.
	DelayUnknown DelayState = iota
	// DelayOnTime means live data was published and the delay is zero.
	DelayOnTime
	// DelayDelayed means live data was published with a nonzero delay.
	DelayDelayed
	// DelaySkipped is a stop-level partial cancellation — the trip itself
	// still runs (DelayCancelled is a separate, trip-level state).
	DelaySkipped
	// DelayCancelled is a trip-level full cancellation.
	DelayCancelled
)

func (s DelayState) String() string {
	switch s {
	case DelayOnTime:
		return "on_time"
	case DelayDelayed:
		return "delayed"
	case DelaySkipped:
		return "skipped"
	case DelayCancelled:
		return "cancelled"
	case DelayUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// StopCall is the realtime state of one stop along a trip, decoded from a
// GTFS-RT TripUpdate.StopTimeUpdate. ArrivalDelay/DepartureDelay are seconds
// (positive = late) and nil when the feed published no live value for that
// event.
type StopCall struct {
	StopID         string
	StopSequence   int
	State          DelayState
	ArrivalDelay   *int
	DepartureDelay *int
}

// TripUpdate is the realtime state of one trip instance, decoded from a
// GTFS-RT TripUpdate entity. TripID is the raw feed identifier: like the
// static feed's trip_id (issue #1390) it is not a stable long-lived key —
// it exists only for the lifetime of one in-memory Snapshot. A later slice
// resolves it against the current static import to display
// trips.trip_short_name; nothing here is persisted.
type TripUpdate struct {
	TripID    string
	RouteID   string
	StartDate string
	// State is the trip-level state: DelayCancelled on a full cancellation,
	// DelayOnTime otherwise. Per-stop delay/skip/unknown state lives on each
	// StopCall.
	State     DelayState
	StopCalls []StopCall
	Timestamp time.Time
}

// Alert is a decoded GTFS-RT service alert (rt/alert), attached to the
// trips/routes/stops it affects.
type Alert struct {
	ID               string
	Cause            string
	Effect           string
	HeaderText       string
	DescriptionText  string
	InformedTripIDs  []string
	InformedRouteIDs []string
	InformedStopIDs  []string
}

// Snapshot is the wholly-replaced current realtime state, rebuilt on every
// poll. It is kept in memory only — nothing here is persisted (issue
// #1393).
type Snapshot struct {
	// Trips is keyed by the raw feed trip_id — see TripUpdate.TripID.
	Trips     map[string]TripUpdate
	Alerts    []Alert
	FetchedAt time.Time
}
