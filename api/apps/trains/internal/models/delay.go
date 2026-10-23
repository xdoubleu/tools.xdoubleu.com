package models

// DelayState is a stop's live state as shown to passengers.
type DelayState int

const (
	// DelayUnknown means no live data is available. Never equivalent to
	// DelayOnTime.
	DelayUnknown DelayState = iota
	// DelayOnTime means live data shows no meaningful delay.
	DelayOnTime
	// DelayDelayed means live data shows a delay.
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

// Alert is a service alert attached to a leg. JSON tags match trains.v1.Alert.
type Alert struct {
	ID              string `json:"id"`
	HeaderText      string `json:"headerText"`
	DescriptionText string `json:"descriptionText"`
}
