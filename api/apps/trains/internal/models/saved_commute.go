package models

import (
	"time"

	"github.com/google/uuid"
)

// SavedCommute is a user's named origin->destination station pair (issue
// #1396). Origin and Destination carry the resolved location_type=1 station
// they point at; OriginStopID/DestinationStopID are the persisted
// S-prefixed UIC parent-station ids.
type SavedCommute struct {
	ID                uuid.UUID
	UserID            string
	Label             string
	OriginStopID      string
	DestinationStopID string
	Position          int
	Origin            Stop
	Destination       Stop
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
