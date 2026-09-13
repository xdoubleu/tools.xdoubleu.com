package feeds

import (
	"context"

	"github.com/google/uuid"
)

// SharedFeed is the reading dashboard's feeds widget payload for one
// subscription: name plus public URL (empty for feed kinds with no public
// URL, e.g. email).
type SharedFeed struct {
	Title string
	URL   string
}

// BuildSharedFeeds assembles the reading dashboard's feeds widget payload
// for a user. It is the only exported entry point feeds' internal item/
// read-state model is reached through from outside this package — the
// dashboard app (api/apps/dashboard) calls this instead of querying feeds
// directly.
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

// SharedItem is a single feed item's summary and read state — the
// single-item counterpart to SharedFeed, used by the learningpaths app
// (api/apps/learningpaths) to resolve a resource linked to a feeds item,
// following the same exported-methods-only cross-app pattern as dashboard.
// It deliberately omits ContentHTML: linking callers need the item's read
// state, not its article body.
type SharedItem struct {
	Title      string
	SourceURL  string
	Read       bool
	Bookmarked bool
}

// GetItemByID looks up a single feed item by ID, scoped to userID. It
// returns database.ErrResourceNotFound both when itemID doesn't exist and
// when it belongs to a different user's feed — GetItem's underlying query
// joins through feeds.feeds on user_id, so a foreign-owned item simply
// doesn't match rather than leaking its existence.
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
