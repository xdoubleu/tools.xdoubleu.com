package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ReadingSourceWeb    = "web"
	ReadingSourceKobo   = "kobo"
	ReadingSourceManual = "manual"
)

// KoboLocation is a Kobo bookmark position, stored as the device sent it.
type KoboLocation struct {
	Source string `json:"Source"`
	Type   string `json:"Type"`
	Value  string `json:"Value"`
}

type BookReadingState struct {
	UserID   string
	BookID   uuid.UUID
	Source   string
	Percent  int
	Location *string
	// KoboLocation is the full Kobo bookmark; nil for non-Kobo writes.
	KoboLocation *KoboLocation
	// ReadAt is when the reading device recorded the position.
	ReadAt    *time.Time
	UpdatedAt time.Time
}
