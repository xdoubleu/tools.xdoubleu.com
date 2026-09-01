package feeds

import "context"

// OpenFeedItems is one feed's count of open (unread) items, for the weekly
// digest job (issue #1355).
type OpenFeedItems struct {
	Title string
	URL   string
	Count int
}

// ListOpenItems returns unread item counts per feed, across all users,
// restricted to feeds with at least one unread item. It is the only
// exported entry point feeds' internal item/read-state model is reached
// through from outside this package for this purpose — the weekly digest
// job (api/internal/observability/jobs, via main.go's
// feedsOpenItemsAdapter) calls this instead of querying feeds.items
// directly.
func (a *Feeds) ListOpenItems(ctx context.Context) ([]OpenFeedItems, error) {
	counts, err := a.Services.Feeds.CountUnreadByFeed(ctx)
	if err != nil {
		return nil, err
	}

	open := make([]OpenFeedItems, 0, len(counts))
	for _, c := range counts {
		open = append(open, OpenFeedItems{
			Title: c.FeedTitle,
			URL:   c.FeedURL,
			Count: c.UnreadCount,
		})
	}
	return open, nil
}
