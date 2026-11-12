package feeds

import (
	"context"
	"net/url"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/services"
)

// SharedFeed is one subscription in the reading dashboard's feeds widget.
// URL is the feed's origin only, since paths, queries and userinfo can carry
// private-feed credentials; it is empty for kinds without one (email).
type SharedFeed struct {
	Title string
	URL   string
}

// BuildSharedFeeds assembles the reading dashboard's feeds widget payload,
// the dashboard app's only entry point into feeds.
func (a *Feeds) BuildSharedFeeds(
	ctx context.Context,
	userID string,
) ([]SharedFeed, error) {
	feeds, err := a.Services.Feeds.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	shared := make([]SharedFeed, 0, len(feeds))
	for _, f := range feeds {
		shared = append(shared, SharedFeed{
			Title: f.Title,
			URL:   feedOrigin(f.URL),
		})
	}

	return shared, nil
}

// feedOrigin returns rawURL's scheme://host, or "" when it isn't an http(s)
// URL.
func feedOrigin(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// SharedItem is a feed item's summary and read state for learningpaths'
// linked resources; it omits the article body.
type SharedItem struct {
	Title      string
	SourceURL  string
	Read       bool
	Bookmarked bool
}

// GetItemByID returns an item scoped to userID, or
// database.ErrResourceNotFound when missing or owned by someone else.
func (a *Feeds) GetItemByID(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) (*SharedItem, error) {
	item, err := a.Services.Feeds.GetItem(ctx, userID, itemID)
	if err != nil {
		return nil, err
	}

	return &SharedItem{
		Title:      item.Title,
		SourceURL:  item.SourceURL,
		Read:       item.ReadAt != nil,
		Bookmarked: item.Bookmarked,
	}, nil
}

// TodayItem is an unread item for the Today page.
type TodayItem struct {
	ID        string
	Title     string
	FeedTitle string
	URL       string
}

// BuildTodayUnread returns the user's unread count and their top unread items,
// the Today page's only entry point into feeds.
func (a *Feeds) BuildTodayUnread(
	ctx context.Context,
	userID string,
	limit int32,
) (int, []TodayItem, error) {
	count, err := a.Services.Feeds.CountUnread(ctx, userID)
	if err != nil {
		return 0, nil, err
	}
	items, _, err := a.Services.Feeds.ListItems(
		ctx, userID, limit, 0, true, nil, false, services.ViewInbox,
	)
	if err != nil {
		return 0, nil, err
	}
	feeds, err := a.Services.Feeds.List(ctx, userID)
	if err != nil {
		return 0, nil, err
	}
	titles := make(map[uuid.UUID]string, len(feeds))
	for _, f := range feeds {
		titles[f.ID] = f.Title
	}

	today := make([]TodayItem, 0, len(items))
	for _, it := range items {
		today = append(today, TodayItem{
			ID:        it.ID.String(),
			Title:     it.Title,
			FeedTitle: titles[it.FeedID],
			URL:       it.SourceURL,
		})
	}
	return count, today, nil
}
