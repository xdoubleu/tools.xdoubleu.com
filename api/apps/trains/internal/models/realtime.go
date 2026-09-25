package models

import "time"

// DelayState is a call's domain state, kept distinct from GTFS-RT
// schedule_relationship: trip-level CANCELED and stop-level SKIPPED/NO_DATA
// mean different things and must not be conflated.
type DelayState int

const (
	// DelayUnknown means no live data was published (the common case). Never
	// equivalent to DelayOnTime.
	DelayUnknown DelayState = iota
	// DelayOnTime means live data shows zero delay.
	DelayOnTime
	// DelayDelayed means live data shows a nonzero delay.
	DelayDelayed
	// DelaySkipped is a stop-level partial cancellation.
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

// StopCall is one stop's realtime state. Delays are seconds (positive =
// late), nil when unpublished.
type StopCall struct {
	StopID         string
	StopSequence   int
	State          DelayState
	ArrivalDelay   *int
	DepartureDelay *int
}

// TripUpdate is one trip's realtime state. TripID is not a stable key; Poll
// resolves it to (trip_short_name, service date). Not persisted.
type TripUpdate struct {
	TripID    string
	RouteID   string
	StartDate string
	// State is trip-level: DelayCancelled or DelayOnTime.
	State     DelayState
	StopCalls []StopCall
	Timestamp time.Time
}

// Alert is a GTFS-RT service alert. JSON tags cover only what trains.v1.Alert
// exposes; Cause/Effect/Informed*IDs are server-side matching detail.
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

// TripKey addresses a trip by trip_short_name and service date ("YYYYMMDD").
// Not trip_id: the static and realtime feeds needn't agree on it.
type TripKey struct {
	ShortName string
	Date      string
}

// Snapshot is the in-memory realtime state, replaced on every poll.
type Snapshot struct {
	// Trips is keyed by (trip_short_name, service date).
	Trips map[TripKey]TripUpdate
	// UnresolvedTripCount counts trip updates with no matching static trip; a
	// sustained nonzero value means the feeds' trip_ids have drifted apart.
	UnresolvedTripCount int
	Alerts              []Alert
	FetchedAt           time.Time
}

// CallFor returns the trip running shortName on serviceDate, which callers
// pass as Europe/Brussels midnight.
func (s Snapshot) CallFor(shortName string, serviceDate time.Time) (TripUpdate, bool) {
	tu, ok := s.Trips[TripKey{
		ShortName: shortName,
		Date:      serviceDate.Format("20060102"),
	}]
	return tu, ok
}
