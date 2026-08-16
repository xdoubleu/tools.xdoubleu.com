package models

import (
	"time"

	"github.com/google/uuid"
)

// FeedSourceRSS/FeedSourceEmail/FeedSourceScrape are the Feed.SourceType
// values. RSS and scrape feeds are both polled (hourly poll-feeds job) —
// scrape feeds have no real feed at their URL, so posts are discovered by
// heuristically scanning the page instead of parsing RSS/Atom (issue #751).
// Email feeds are populated by the Resend inbound-webhook push (issue #595)
// and are never polled.
const (
	FeedSourceRSS    = "rss"
	FeedSourceEmail  = "email"
	FeedSourceScrape = "scrape"
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
	// ConsecutiveFailures counts unbroken poll failures; reset to 0 on any
	// success (issue #799).
	ConsecutiveFailures int
	// NotifiedAt marks an unresolved problem email already sent for this
	// feed (error streak or quiet-feed detection); cleared on recovery so
	// at most one notification is outstanding at a time (issue #799).
	NotifiedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Item is one ingested feed entry (feeds.items). A feed and its items only
// ever belong to one user, so there is no per-user junction table — read/
// bookmarked/dismissed state lives directly as columns here.
type Item struct {
	ID        uuid.UUID
	FeedID    uuid.UUID
	GUID      string
	Title     string
	SourceURL string
	// ContentHTML is the extracted article body. It is only populated by the
	// single-item read (GetByIDForUser) — list and update queries leave it
	// empty and set HasContent instead, so a page of items never drags every
	// article body out of the database (issue #1027).
	ContentHTML string
	// HasContent reports whether ContentHTML is non-empty, without reading
	// it. Only set by the queries using itemListColumns.
	HasContent bool
	// PublishedAt is the item's true publish date from the feed/email, used
	// for ordering instead of ingest time.
	PublishedAt time.Time
	// ReadAt is nil while unread.
	ReadAt *time.Time
	// Dismissed hides the item from the default view without deleting it.
	Dismissed  bool
	Bookmarked bool
	// ReadProgressPct is the furthest scroll position reached in the reader
	// (0-100), monotonic — re-opening and scrolling less never lowers it
	// (issue #798).
	ReadProgressPct int
	// IngestError holds the most recent ingest failure for this item, if any
	// (e.g. its linked page could not be fetched/extracted); the item is
	// still tracked (guid marked seen) so it is never retried automatically.
	IngestError *string
	CreatedAt   time.Time
}

// FeedStats aggregates one feed's posting cadence and read/completion
// metrics (issue #798).
type FeedStats struct {
	FeedID    uuid.UUID
	FeedTitle string
	ItemCount int
	// AvgIntervalHours is the mean gap between consecutive items' published_at,
	// 0 when the feed has fewer than 2 items.
	AvgIntervalHours float64
	// ReadRate is the fraction (0-1) of items with read_at set.
	ReadRate float64
	// AvgReadProgressPct is the mean furthest-scroll-reached across items.
	AvgReadProgressPct float64
}

// DayCount is one bucket of a day → ingested-item-count histogram (issue
// #798).
type DayCount struct {
	Day   time.Time
	Count int
}
