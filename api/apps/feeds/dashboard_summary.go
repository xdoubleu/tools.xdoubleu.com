package feeds

import (
	"context"
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
