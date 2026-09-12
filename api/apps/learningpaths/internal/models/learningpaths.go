package models

import (
	"time"

	"github.com/google/uuid"
)

type LearningPath struct {
	ID        uuid.UUID
	UserID    string
	Title     string
	Goal      string
	Routine   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Modules   []Module
	Resources []Resource
}

type Module struct {
	ID             uuid.UUID
	LearningPathID uuid.UUID
	Title          string
	SortOrder      int
	Items          []Item
}

type Item struct {
	ID          uuid.UUID
	ModuleID    uuid.UUID
	Type        string
	Description string
	SortOrder   int
	Completed   bool
}

type Resource struct {
	ID               uuid.UUID
	LearningPathID   uuid.UUID
	Text             string
	SortOrder        int
	LinkedBookID     *uuid.UUID
	LinkedFeedItemID *uuid.UUID

	// LinkedBook/LinkedFeedItem hold the resolved, read-only state of
	// LinkedBookID/LinkedFeedItemID — populated by
	// LearningPathService.Get when the link still resolves for the calling
	// user, nil otherwise (including when the linked book/feed item has
	// since been removed). Never persisted; always nil coming out of the
	// repository layer.
	LinkedBook     *LinkedBook
	LinkedFeedItem *LinkedFeedItem
}

// LinkedBook is the resolved state of a resource linked to a books library
// entry (#1474): enough to show the entry's own reading progress alongside
// the path item, without learningpaths depending on books' proto/internal
// types directly.
type LinkedBook struct {
	Title           string
	Status          string
	ProgressPercent int
	CoverURL        string
}

// LinkedFeedItem is the resolved state of a resource linked to a feeds
// item (#1474) — same purpose as LinkedBook.
type LinkedFeedItem struct {
	Title      string
	SourceURL  string
	Read       bool
	Bookmarked bool
}

// ItemForTask is the minimal projection SendItemToTodoist needs to build a
// Todoist task's content — the item itself plus its owning path's title for
// context, not the full tree (issue #1475).
type ItemForTask struct {
	Item      Item
	PathTitle string
}
