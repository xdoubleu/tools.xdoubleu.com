package feeds

import (
	"context"

	"github.com/google/uuid"
)

// SharedFeed is one subscription in the reading dashboard's feeds widget;
// URL is empty for kinds without one (email).
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
			URL:   f.URL,
		})
	}

	return shared, nil
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
