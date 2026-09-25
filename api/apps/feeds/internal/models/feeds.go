package models

import (
	"time"

	"github.com/google/uuid"
)

// FeedSourceRSS/Email/Scrape are the Feed.SourceType values. RSS and scrape
// feeds are polled hourly (scrape feeds discover posts heuristically); email
// feeds are populated by the Resend inbound webhook and never polled.
const (
	FeedSourceRSS    = "rss"
	FeedSourceEmail  = "email"
	FeedSourceScrape = "scrape"
)

// Feed is an RSS/Atom, scrape or email-relay subscription (feeds.feeds).
type Feed struct {
	ID     uuid.UUID
	UserID string
	URL    string
	Title  string
	// SourceType is one of the FeedSource* constants.
	SourceType string
	// InboundToken is the SHA-256 hash of the email alias token; nil for
	// non-email feeds. The plaintext is returned once, at creation.
	InboundToken *string
	// ETag / LastModified are conditional-GET validators; nil until fetched.
	ETag          *string
	LastModified  *string
	LastFetchedAt *time.Time
	// LastError is the latest poll failure; nil when the last poll succeeded.
	LastError *string
	// ConsecutiveFailures counts unbroken poll failures.
	ConsecutiveFailures int
	// NotifiedAt marks an outstanding problem email; cleared on recovery so
	// at most one is outstanding.
	NotifiedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Item is one ingested feed entry (feeds.items). Feeds are single-user, so
// read/bookmarked/dismissed state lives here.
type Item struct {
	ID        uuid.UUID
	FeedID    uuid.UUID
	GUID      string
	Title     string
	SourceURL string
	// ContentHTML is the article body, populated only by the single-item
	// read; list and update queries set HasContent instead.
	ContentHTML string
	// HasContent reports whether ContentHTML is non-empty, without reading it.
	HasContent bool
	// PublishedAt is the true publish date, used for ordering.
	PublishedAt time.Time
	// ReadAt is nil while unread.
	ReadAt *time.Time
	// Dismissed hides the item from the default view without deleting it.
	Dismissed  bool
	Bookmarked bool
	// ReadProgressPct is the furthest scroll reached (0-100), monotonic.
	ReadProgressPct int
	// IngestError is the latest ingest failure; the guid stays seen so it
	// is never retried automatically.
	IngestError *string
	CreatedAt   time.Time
}

// FeedUnreadCount is one feed's open (unread, non-dismissed, ingested) item
// count.
type FeedUnreadCount struct {
	FeedID      uuid.UUID
	FeedTitle   string
	FeedURL     string
	UnreadCount int
}

// FeedStats aggregates one feed's posting cadence and read metrics.
type FeedStats struct {
	FeedID    uuid.UUID
	FeedTitle string
	ItemCount int
	// AvgIntervalHours is the mean gap between items; 0 below 2 items.
	AvgIntervalHours float64
	// ReadRate is the fraction (0-1) of items with read_at set.
	ReadRate float64
	// AvgReadProgressPct is the mean furthest-scroll-reached across items.
	AvgReadProgressPct float64
}

// DayCount is one bucket of a per-day ingested-item histogram.
type DayCount struct {
	Day   time.Time
	Count int
}
