package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// ErrInvalidJourneyID is returned by DecodeJourneyID for an id it can't decode.
var ErrInvalidJourneyID = errors.New("trains: invalid journey id")

// LegRef is what a journey_id round-trips through the client to rebuild one
// leg. Holds trip_short_name, never the churning trip_id.
type LegRef struct {
	TripShortName string    `json:"s"`
	BoardStopID   string    `json:"b"`
	BoardTime     time.Time `json:"bt"`
	AlightStopID  string    `json:"a"`
	AlightTime    time.Time `json:"at"`
}

// EncodeJourneyID builds the opaque id DecodeJourneyID reverses.
func EncodeJourneyID(legs []LegRef) string {
	data, err := json.Marshal(legs)
	if err != nil {
		// legs are in-process values, so this can't fail in practice.
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeJourneyID reverses EncodeJourneyID, validating it as user input.
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
