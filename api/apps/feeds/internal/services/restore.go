package services

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// ErrItemNotFiltered is returned when restoring an item no rule filters.
var ErrItemNotFiltered = errors.New("item is not filtered")

// restoreFetchTimeout keeps RestoreItem's content fetch under kamal-proxy's
// response timeout (adr-0017); a timed-out fetch restores without content.
const restoreFetchTimeout = 20 * time.Second

// RestoreItem returns one of the user's filtered items to the inbox as
// unread at its original date; no rule filters it again. An item without
// content gets its linked page fetched as ingest would; a failed fetch still
// restores it, without content.
func (s *FeedService) RestoreItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) (*models.Item, error) {
	item, err := s.items.GetByIDForUser(ctx, userID, itemID)
	if err != nil {
		return nil, err
	}
	if item.FilteredAt == nil {
		return nil, ErrItemNotFiltered
	}

	content := ""
	if item.ContentHTML == "" {
		content = s.fetchRestoredContent(ctx, *item)
	}
	return s.items.Restore(ctx, userID, itemID, content)
}

// fetchRestoredContent fetches a filtered item's linked page, the fetch
// ingest skipped for it; email items have no page to fetch.
func (s *FeedService) fetchRestoredContent(
	ctx context.Context,
	item models.Item,
) string {
	u, err := url.Parse(item.SourceURL)
	if err != nil || !isHTTPScheme(u.Scheme) {
		return ""
	}
	fetchCtx, cancel := context.WithTimeout(ctx, restoreFetchTimeout)
	defer cancel()
	title := item.Title
	return s.fetchLinkedPageHTML(fetchCtx, item.SourceURL, &title)
}
