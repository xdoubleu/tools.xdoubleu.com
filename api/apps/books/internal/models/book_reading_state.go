package models

import (
	"encoding/json"
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

// ReadingPosition is a format-neutral position: Href (content document path
// relative to the EPUB zip root) plus Offset (character offset into the
// section body's text) for EPUB/KEPUB, or a 1-based Page for PDF.
type ReadingPosition struct {
	Href   string `json:"href,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Page   int    `json:"page,omitempty"`
}

// MarshalJSON stores {"page"} for a PDF position, else {"href","offset"}.
func (p ReadingPosition) MarshalJSON() ([]byte, error) {
	if p.Page > 0 {
		return json.Marshal(struct {
			Page int `json:"page"`
		}{p.Page})
	}
	return json.Marshal(struct {
		Href   string `json:"href"`
		Offset int    `json:"offset"`
	}{p.Href, p.Offset})
}

type BookReadingState struct {
	UserID   string
	BookID   uuid.UUID
	Source   string
	Percent  int
	Location *string
	// KoboLocation is the full Kobo bookmark; nil for non-Kobo writes.
	KoboLocation *KoboLocation
	// Position is the neutral position; nil for Kobo and percent-only writes.
	Position *ReadingPosition
	// ReadAt is when the reading device recorded the position; newest wins.
	ReadAt    *time.Time
	UpdatedAt time.Time
}
