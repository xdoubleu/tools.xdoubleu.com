package models

import (
	"time"

	"github.com/google/uuid"
)

// Feed is an RSS/Atom subscription (reading.feeds). Items ingested from a
// feed become regular catalog rows with CategoryRSS; KoboSync auto-enables
// Kobo sync for every newly ingested item.
type Feed struct {
	ID       uuid.UUID
	UserID   string
	URL      string
	Title    string
	KoboSync bool
	// ETag / LastModified are the conditional-GET validators from the last
	// successful fetch; nil until the feed has been fetched once.
	ETag          *string
	LastModified  *string
	LastFetchedAt *time.Time
	// LastError holds the most recent poll failure, nil when the last poll
	// succeeded.
	LastError *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// FeedItemBook links a library book to the feed it was ingested from, for
// labeling the ad hoc feed-reader view (issue #476).
type FeedItemBook struct {
	BookID    uuid.UUID
	FeedID    uuid.UUID
	FeedTitle string
}
