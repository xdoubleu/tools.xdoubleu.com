package feeds

import "context"

// OpenFeedItems is one feed's count of unread items.
type OpenFeedItems struct {
	Title string
	URL   string
	Count int
}

// ListOpenItems returns unread item counts per feed across all users,
// omitting feeds with none; used by the weekly digest job.
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
