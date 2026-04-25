package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusToRead  = "to-read"
	StatusReading = "currently-reading"
	StatusRead    = "read"
	StatusDropped = "dropped"
)

const (
	TagOwnPhysical = "own-physical"
	TagOwnDigital  = "own-digital"
	TagFavourite   = "favourite"
)

// IsSpecialTag reports whether a tag has reserved UI treatment (ownership, favourite).
func IsSpecialTag(t string) bool {
	return t == TagOwnPhysical || t == TagOwnDigital || t == TagFavourite
}

type Book struct {
	ID           uuid.UUID
	Title        string
	Authors      []string
	ISBN13       *string
	ISBN10       *string
	CoverURL     *string
	Description  *string
	ExternalRefs map[string]string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HasTag reports whether the user_book has the given tag.
func (ub UserBook) HasTag(tag string) bool {
	for _, t := range ub.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// DisplayTags returns only non-special tags for UI badge rendering.
func (ub UserBook) DisplayTags() []string {
	out := make([]string, 0, len(ub.Tags))
	for _, t := range ub.Tags {
		if !IsSpecialTag(t) {
			out = append(out, t)
		}
	}
	return out
}

type UserBook struct {
	ID             uuid.UUID
	UserID         string
	BookID         uuid.UUID
	Book           *Book
	Status         string
	Tags           []string
	ShelfPositions map[string]int
	Rating         *int16
	Notes          *string
	FinishedAt     []time.Time
	AddedAt        time.Time
	UpdatedAt      time.Time
}
