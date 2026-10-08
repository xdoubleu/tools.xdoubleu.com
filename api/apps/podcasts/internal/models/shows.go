package models

import (
	"time"

	"github.com/google/uuid"
)

// Show is a podcast in a user's favourites.
type Show struct {
	ID         uuid.UUID
	UserID     string
	ITunesID   int64
	Title      string
	Author     string
	ArtworkURL string
	FeedURL    string
	AppleURL   string
	AddedAt    time.Time
}
