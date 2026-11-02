package feeds

import (
	"context"
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// digestItemLimit caps each weekly-digest section.
const digestItemLimit = 5

// DigestItem is an unread item listed in the weekly digest.
type DigestItem struct {
	Title     string
	FeedTitle string
	URL       string
	ExpiresAt *time.Time
}

// DigestItems are the weekly digest's two sections.
type DigestItems struct {
	Newest     []DigestItem
	LastChance []DigestItem
}

// ListDigestItems returns the newest unread items and the ones about to
// expire, across all users; used by the weekly digest job.
func (a *Feeds) ListDigestItems(ctx context.Context) (DigestItems, error) {
	newest, lastChance, err := a.Services.Feeds.DigestItems(ctx, digestItemLimit)
	if err != nil {
		return DigestItems{}, err
	}
	convert := func(in []models.DigestItem) []DigestItem {
		out := make([]DigestItem, len(in))
		for i, d := range in {
			out[i] = DigestItem(d)
		}
		return out
	}
	return DigestItems{Newest: convert(newest), LastChance: convert(lastChance)}, nil
}
