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

	// ETag and LastModified arm the next conditional feed fetch.
	ETag         *string
	LastModified *string
	// FetchError is the last fetch's error; empty when it succeeded.
	FetchError string
}

// Episode is one entry of a show's feed.
type Episode struct {
	ID              uuid.UUID
	ShowID          uuid.UUID
	GUID            string
	Title           string
	Summary         string
	Link            string
	AudioURL        string
	DurationSeconds *int
	PublishedAt     *time.Time
}

// ListedEpisode is an episode with the show it belongs to.
type ListedEpisode struct {
	Episode
	ShowTitle  string
	ArtworkURL string
	AppleURL   string
}
