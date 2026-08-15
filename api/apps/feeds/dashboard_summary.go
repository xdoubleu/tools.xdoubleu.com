package feeds

import (
	"context"
	"time"
)

// SummaryItem is a lightweight, read-only projection of one feed item for
// the reading dashboard's feeds widget — no read/dismissed/bookmarked
// state, which is not meaningful outside the feeds app itself.
type SummaryItem struct {
	Title       string
	SourceURL   string
	PublishedAt time.Time
}

// Summary is the reading dashboard's feeds widget payload: an unread count
// across all of the user's feeds, plus a handful of the most recent unread
// items.
type Summary struct {
	UnreadCount int
	Items       []SummaryItem
}

// BuildSummary assembles the reading dashboard's feeds widget payload for a
// user. It is the only exported entry point feeds' internal item/read-state
// model is reached through from outside this package — the dashboard app
// (api/apps/dashboard) calls this instead of querying feeds.items directly.
func (a *Feeds) BuildSummary(
	ctx context.Context,
	userID string,
	limit int,
) (*Summary, error) {
	unreadCount, err := a.Services.Feeds.CountUnread(ctx, userID)
	if err != nil {
		return nil, err
	}

	//nolint:gosec // limit is caller-controlled (dashboard widget size), not user input
	items, _, err := a.Services.Feeds.ListItems(
		ctx, userID, int32(limit), 0, true, nil, false,
	)
	if err != nil {
		return nil, err
	}

	summaryItems := make([]SummaryItem, 0, len(items))
	for _, item := range items {
		summaryItems = append(summaryItems, SummaryItem{
			Title:       item.Title,
			SourceURL:   item.SourceURL,
			PublishedAt: item.PublishedAt,
		})
	}

	return &Summary{
		UnreadCount: unreadCount,
		Items:       summaryItems,
	}, nil
}
