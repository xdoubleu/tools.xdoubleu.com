package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// maxJourneyLegs bounds the iRail calls one decoded journey_id can trigger.
const maxJourneyLegs = 8

// ErrInvalidJourneyID is returned by DecodeJourneyID for an id it can't decode.
var ErrInvalidJourneyID = errors.New("trains: invalid journey id")

// LegRef is what a journey_id round-trips through the client to rebuild one
// leg: the iRail vehicle id and the boarded stretch of its run.
type LegRef struct {
	VehicleID    string    `json:"v"`
	BoardStopID  string    `json:"b"`
	BoardTime    time.Time `json:"bt"`
	AlightStopID string    `json:"a"`
	AlightTime   time.Time `json:"at"`
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
	if len(legs) == 0 || len(legs) > maxJourneyLegs {
		return nil, ErrInvalidJourneyID
	}
	for _, l := range legs {
		if !strings.HasPrefix(l.VehicleID, "BE.NMBS.") || l.BoardStopID == "" ||
			l.AlightStopID == "" {
			return nil, ErrInvalidJourneyID
		}
	}
	return legs, nil
}
