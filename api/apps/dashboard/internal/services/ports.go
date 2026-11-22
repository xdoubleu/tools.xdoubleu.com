package services

import (
	"context"
	"time"

	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Today's list blocks are capped at this many entries.
const TodayLimit = 3

// ReadingBook is a book being read.
type ReadingBook struct {
	ID              string
	Title           string
	Authors         []string
	ProgressPercent int
}

// FeedItem is an unread feed item.
type FeedItem struct {
	ID        string
	Title     string
	FeedTitle string
	URL       string
}

// Meal is one planned meal.
type Meal struct {
	Date string // yyyy-MM-dd
	Slot string
	Name string
}

// WatchingTitle is a title being watched.
type WatchingTitle struct {
	ID           string
	Title        string
	HasNewSeason bool
}

// LearningPath is an active learning path.
type LearningPath struct {
	ID    string
	Title string
}

// AttentionTask is a board task waiting on a person.
type AttentionTask struct {
	Ref    string
	Title  string
	Status string
}

// ReadingSource lists the books a user is reading.
type ReadingSource interface {
	ListCurrentlyReading(
		ctx context.Context, userID string, limit int,
	) ([]ReadingBook, error)
}

// FeedsSource summarises a user's unread feed items.
type FeedsSource interface {
	BuildTodayUnread(
		ctx context.Context, userID string, limit int32,
	) (int, []FeedItem, error)
}

// MealsSource lists a user's meals around today.
type MealsSource interface {
	ListTodayMeals(ctx context.Context, userID string, today time.Time) ([]Meal, error)
}

// ShoppingSource counts a user's shopping list.
type ShoppingSource interface {
	CountCustomListItems(ctx context.Context, userID string) (int, error)
}

// WatchingSource lists the titles a user is watching.
type WatchingSource interface {
	ListWatching(ctx context.Context, userID string, limit int32) ([]WatchingTitle, error)
}

// LearningSource lists a user's active learning paths.
type LearningSource interface {
	ListActivePaths(
		ctx context.Context, userID string, limit int32,
	) ([]LearningPath, error)
}

// AttentionSource lists the board tasks waiting on a person.
type AttentionSource interface {
	ListAttentionTasks(ctx context.Context, limit int) ([]AttentionTask, error)
}

// ShareTokens resolves a share token of a kind to its owner.
type ShareTokens interface {
	ResolveToken(
		ctx context.Context,
		token string,
		kind sharedmodels.DashboardKind,
	) (userID string, displayName string, err error)
}
