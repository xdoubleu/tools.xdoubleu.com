package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// feedsStore is the feed storage surface FeedService needs.
type feedsStore interface {
	List(ctx context.Context, userID string) ([]models.Feed, error)
	ListAll(ctx context.Context) ([]models.Feed, error)
	ListUnhealthy(ctx context.Context) ([]models.Feed, error)
	GetByID(ctx context.Context, userID string, id uuid.UUID) (*models.Feed, error)
	GetByInboundTokenHash(ctx context.Context, hash string) (*models.Feed, error)
	Insert(ctx context.Context, feed models.Feed) (*models.Feed, error)
	Update(
		ctx context.Context,
		userID string,
		id uuid.UUID,
		title string,
		expiryDays *int,
	) error
	Delete(ctx context.Context, userID string, id uuid.UUID) error
	SetFetchResult(
		ctx context.Context,
		id uuid.UUID,
		etag, lastModified, fetchErr *string,
	) (*models.Feed, error)
	MarkNotified(ctx context.Context, id uuid.UUID) error
	ClearNotified(ctx context.Context, id uuid.UUID) error
}

// itemsStore is the item storage surface FeedService needs.
type itemsStore interface {
	CountUnread(ctx context.Context, userID string) (int, error)
	DigestLastChance(
		ctx context.Context, limit int, window time.Duration,
	) ([]models.DigestItem, error)
	DigestNewest(ctx context.Context, limit int) ([]models.DigestItem, error)
	ExistingTitles(
		ctx context.Context, feedID uuid.UUID, titles []string,
	) (map[string]bool, error)
	ExpireDue(ctx context.Context) (int64, error)
	FillMissingCategories(
		ctx context.Context, feedID uuid.UUID, categories map[string][]string,
	) error
	FilterNewGUIDs(
		ctx context.Context, feedID uuid.UUID, guids []string,
	) ([]string, error)
	GetByIDForUser(
		ctx context.Context, userID string, itemID uuid.UUID,
	) (*models.Item, error)
	Insert(ctx context.Context, item models.Item) error
	InsertMany(ctx context.Context, items []models.Item) error
	ItemsPerDay(
		ctx context.Context, userID string, since time.Time,
	) ([]models.DayCount, error)
	ListByUser(
		ctx context.Context,
		userID string,
		limit, offset int32,
		unreadOnly bool,
		feedID *uuid.UUID,
		bookmarkedOnly bool,
	) ([]models.Item, bool, error)
	ListEngaged(ctx context.Context, userID string) ([]models.EngagedItem, error)
	ListExpiredByUser(
		ctx context.Context,
		userID string,
		limit, offset int32,
		feedID *uuid.UUID,
	) ([]models.Item, bool, error)
	ListFilteredByUser(
		ctx context.Context,
		userID string,
		limit, offset int32,
		feedID *uuid.UUID,
	) ([]models.Item, bool, error)
	ListLastChance(
		ctx context.Context,
		userID string,
		limit, offset int32,
		window time.Duration,
		feedID *uuid.UUID,
	) ([]models.Item, bool, error)
	ListRankingCandidates(
		ctx context.Context, userID string, readSince time.Time, limit int,
	) ([]models.Item, error)
	ReadStats(
		ctx context.Context, userID string, since time.Time,
	) (map[uuid.UUID]models.ReadStat, map[string]models.ReadStat, error)
	RecentPublishedAt(
		ctx context.Context, feedID uuid.UUID, limit int,
	) ([]time.Time, error)
	Restore(
		ctx context.Context, userID string, itemID uuid.UUID, contentHTML string,
	) (*models.Item, error)
	RestoreExpired(
		ctx context.Context, userID string, itemID uuid.UUID,
	) (*models.Item, error)
	Stats(ctx context.Context, userID string) ([]models.FeedStats, error)
	Update(
		ctx context.Context,
		userID string,
		itemID uuid.UUID,
		read, dismissed, bookmarked *bool,
		readProgressPct *int32,
	) (*models.Item, error)
}

// filterRulesStore is the filter-rule storage surface FeedService needs.
type filterRulesStore interface {
	Create(
		ctx context.Context,
		userID string,
		feedID *uuid.UUID,
		kind, value string,
	) (*models.FilterRule, error)
	Delete(ctx context.Context, userID string, id uuid.UUID) error
	DismissSuggestion(
		ctx context.Context, userID string, feedID uuid.UUID, category string,
	) error
	ListByUser(ctx context.Context, userID string) ([]models.FilterRule, error)
	ListForFeed(
		ctx context.Context, userID string, feedID uuid.UUID,
	) ([]models.FilterRule, error)
	ListSuggestions(
		ctx context.Context, userID string, c models.SuggestionCriteria,
	) ([]models.FilterRuleSuggestion, error)
}

// userLookup resolves a feed owner for problem emails.
type userLookup interface {
	GetByID(ctx context.Context, id string) (*sharedmodels.User, error)
}

// ReceivedMail fetches the body of an email Resend received for an alias.
type ReceivedMail interface {
	// HTML returns the email's HTML body, or its text part wrapped for display.
	HTML(ctx context.Context, emailID string) (string, error)
}
