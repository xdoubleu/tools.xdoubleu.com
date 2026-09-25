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

	// LinkedBook/LinkedFeedItem are resolved by LearningPathService.Get when
	// the link still resolves for the caller, else nil. Never persisted.
	LinkedBook     *LinkedBook
	LinkedFeedItem *LinkedFeedItem
}

// LinkedBook is the resolved state of a resource linked to a books entry,
// independent of books' own types.
type LinkedBook struct {
	Title           string
	Status          string
	ProgressPercent int
	CoverURL        string
}

// LinkedFeedItem is the resolved state of a resource linked to a feeds item.
type LinkedFeedItem struct {
	Title      string
	SourceURL  string
	Read       bool
	Bookmarked bool
}

// ItemForTask is an item plus its path's title, for building a Todoist task.
type ItemForTask struct {
	Item      Item
	PathTitle string
}
