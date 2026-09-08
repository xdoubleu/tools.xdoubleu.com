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
// it exists only long enough for RealtimeService.Poll to resolve it against
// the current static import into a (trip_short_name, service date) pair, the
// coordinates everything downstream addresses a train by. Nothing here is
// persisted.
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
// trips/routes/stops it affects. JSON tags cover only the fields
// trains.v1.Alert also exposes — the websocket push (this package's own
// JSON DTO) and the GetJourneyDetail RPC response decode into the same
// client-side shape (issue #1394); Cause/Effect/Informed*IDs are
// server-side matching detail, not shown to a passenger.
type Alert struct {
	ID               string   `json:"id"`
	Cause            string   `json:"-"`
	Effect           string   `json:"-"`
	HeaderText       string   `json:"headerText"`
	DescriptionText  string   `json:"descriptionText"`
	InformedTripIDs  []string `json:"-"`
	InformedRouteIDs []string `json:"-"`
	InformedStopIDs  []string `json:"-"`
}

// TripKey addresses one trip instance by the coordinates that survive a
// daily feed churn: its trips.trip_short_name and its GTFS service date
// ("YYYYMMDD"). The raw trip_id is deliberately not part of it — the static
// and GTFS-RT feeds are ingested from separate BMC endpoints and nothing
// guarantees they assign the same trip_id to the same physical train
// (issue #1484).
type TripKey struct {
	ShortName string
	Date      string
}

// Snapshot is the wholly-replaced current realtime state, rebuilt on every
// poll. It is kept in memory only — nothing here is persisted (issue
// #1393).
type Snapshot struct {
	// Trips is keyed by (trip_short_name, service date) — RealtimeService.Poll
	// resolves each decoded TripUpdate's raw trip_id against the current
	// static import before storing it here.
	Trips map[TripKey]TripUpdate
	// UnresolvedTripCount is how many decoded trip updates in this poll cycle
	// carried a trip_id with no matching static trip — normally zero; a
	// sustained nonzero value means the two feeds' trip_id namespaces have
	// drifted apart (issue #1484).
	UnresolvedTripCount int
	Alerts              []Alert
	FetchedAt           time.Time
}

// CallFor returns the realtime state of the trip running shortName on
// serviceDate, if the last poll cycle published one. serviceDate is read in
// its own location, so callers pass the feed-local (Europe/Brussels)
// midnight of the service day.
func (s Snapshot) CallFor(shortName string, serviceDate time.Time) (TripUpdate, bool) {
	tu, ok := s.Trips[TripKey{
		ShortName: shortName,
		Date:      serviceDate.Format("20060102"),
	}]
	return tu, ok
}
