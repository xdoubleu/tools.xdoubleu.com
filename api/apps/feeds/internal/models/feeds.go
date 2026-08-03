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
// (feeds.feeds). Its items are stored directly on feeds.items — they are not
// library entries and never reference any other app's schema.
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

// Item is one ingested feed entry (feeds.items). A feed and its items only
// ever belong to one user, so there is no per-user junction table — read/
// favourite/dismissed state lives directly as columns here.
type Item struct {
	ID          uuid.UUID
	FeedID      uuid.UUID
	GUID        string
	Title       string
	SourceURL   string
	ContentHTML string
	// PublishedAt is the item's true publish date from the feed/email, used
	// for ordering instead of ingest time.
	PublishedAt time.Time
	// ReadAt is nil while unread.
	ReadAt *time.Time
	// Dismissed hides the item from the default view without deleting it.
	Dismissed bool
	Favourite bool
	// IngestError holds the most recent ingest failure for this item, if any
	// (e.g. its linked page could not be fetched/extracted); the item is
	// still tracked (guid marked seen) so it is never retried automatically.
	IngestError *string
	CreatedAt   time.Time
}
