package models

import (
	"time"

	"github.com/google/uuid"
)

// SavedCommute is a user's named origin->destination station pair, stored
// as UIC station ids.
type SavedCommute struct {
	ID                uuid.UUID
	UserID            string
	Label             string
	OriginStopID      string
	DestinationStopID string
	Position          int
	Origin            Station
	Destination       Station
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
