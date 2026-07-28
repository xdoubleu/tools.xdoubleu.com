package models

import (
	"time"

	"github.com/google/uuid"
)

// FeedSourceRSS/FeedSourceEmail are the Feed.SourceType values. RSS feeds are
// polled (pkg gofeed, hourly poll-feeds job); email feeds are populated by
// the Resend inbound-webhook push (issue #595) and are never polled.
const (
	FeedSourceRSS   = "rss"
	FeedSourceEmail = "email"
)

// Feed is an RSS/Atom subscription or an email-relay newsletter subscription
// (reading.feeds). Items ingested from a feed become regular catalog rows
// with CategoryRSS. RSS items never sync to Kobo devices (issue #640).
type Feed struct {
	ID     uuid.UUID
	UserID string
	URL    string
	Title  string
	// SourceType is FeedSourceRSS or FeedSourceEmail.
	SourceType string
	// InboundToken is the SHA-256 hash of the per-feed inbound email alias's
	// token; nil for rss feeds. The plaintext token is never stored — it is
	// returned to the caller once, at creation time.
	InboundToken *string
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
