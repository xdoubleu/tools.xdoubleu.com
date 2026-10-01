package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// FullProgressPercent is the reading progress at which a book-linked item is
// complete.
const FullProgressPercent = 100

// LearnShelf is the books shelf an item's ExternalBook is added to.
const LearnShelf = "To Learn"

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
	// ReminderSchedules maps a normalized item type to the Todoist due string
	// of its reminder tasks.
	ReminderSchedules map[string]string
	// Paused stops the path's Todoist reminders; plan and progress are kept.
	Paused bool

	// TodoistProjectID is the path's own Todoist project, created on first
	// sync. Never sent over the wire.
	TodoistProjectID *string
}

type Module struct {
	ID             uuid.UUID
	LearningPathID uuid.UUID
	Title          string
	SortOrder      int
	Items          []Item
	Quiz           []QuizQuestion
}

// QuizQuestion is one multiple-choice item in a module's quiz.
type QuizQuestion struct {
	Prompt             string   `json:"prompt"`
	Options            []string `json:"options"`
	CorrectAnswerIndex int      `json:"correct_answer_index"`
}

type Item struct {
	ID          uuid.UUID
	ModuleID    uuid.UUID
	Type        string
	Description string
	SortOrder   int
	Completed   bool
	// Due is the item's Todoist due string (e.g. "every day at 20:00"),
	// overriding the path's schedule for its type.
	Due string
	// LinkedBookID pins a book-linked item; its Completed is derived on read.
	LinkedBookID *uuid.UUID

	// ExternalBook, on Create/Update only, is resolved into LinkedBookID by
	// adding the book to the library. Never persisted.
	ExternalBook *ExternalBookRef

	// LinkedBook is resolved by LearningPathService.Get when the link still
	// resolves for the caller, else nil. Never persisted.
	LinkedBook *LinkedBook

	// TodoistTaskID is the id of the item's task in Todoist while its module
	// is active; empty/nil means no task is expected. Never sent over the wire.
	TodoistTaskID *string
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

// ExternalBookRef is a books external search result.
type ExternalBookRef struct {
	Provider   string
	ProviderID string
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

// NormalizeItemType is the key an item type has in ReminderSchedules.
func NormalizeItemType(itemType string) string {
	return strings.ToLower(strings.TrimSpace(itemType))
}

// NormalizeReminderSchedules keys schedules by normalized type and drops
// blank entries.
func NormalizeReminderSchedules(schedules map[string]string) map[string]string {
	result := make(map[string]string, len(schedules))
	for itemType, due := range schedules {
		key, value := NormalizeItemType(itemType), strings.TrimSpace(due)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

// ItemDue is the due string of its reminder task: its own Due, else lp's
// schedule for its type, else "" (undated).
func (lp *LearningPath) ItemDue(it Item) string {
	if it.Due != "" {
		return it.Due
	}
	return lp.ReminderSchedules[NormalizeItemType(it.Type)]
}
