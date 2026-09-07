package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// ErrInvalidJourneyID is returned by DecodeJourneyID when id isn't one this
// process (or an earlier deploy using the same encoding) produced.
var ErrInvalidJourneyID = errors.New("trains: invalid journey id")

// LegRef is the information needed to reconstruct one leg's full detail
// without persisting anything server-side: a Journey's opaque journey_id
// (see EncodeJourneyID) is just this data, round-tripped through the
// client. trip_id itself is never part of it — only trip_short_name, which
// is resolved back to the day's trip_id on read (issue #1394, following the
// trip_id-churn invariant from #1390).
type LegRef struct {
	TripShortName string    `json:"s"`
	BoardStopID   string    `json:"b"`
	BoardTime     time.Time `json:"bt"`
	AlightStopID  string    `json:"a"`
	AlightTime    time.Time `json:"at"`
}

// EncodeJourneyID builds the opaque, self-describing id a Journey is
// returned with — decodable back into its LegRefs by DecodeJourneyID with no
// server-side storage.
func EncodeJourneyID(legs []LegRef) string {
	data, err := json.Marshal(legs)
	if err != nil {
		// legs is always built from in-process values (never user input), so
		// this cannot fail in practice.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeJourneyID reverses EncodeJourneyID. It rejects anything that isn't
// valid base64/JSON or carries no legs, since a client-supplied journey_id
// is otherwise unvalidated user input.
func DecodeJourneyID(id string) ([]LegRef, error) {
	data, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return nil, ErrInvalidJourneyID
	}
	var legs []LegRef
	if err = json.Unmarshal(data, &legs); err != nil {
		return nil, ErrInvalidJourneyID
	}
	if len(legs) == 0 {
		return nil, ErrInvalidJourneyID
	}
	return legs, nil
}
