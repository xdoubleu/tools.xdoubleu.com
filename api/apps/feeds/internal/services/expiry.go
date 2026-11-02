package services

import (
	"context"
	"errors"
	"time"

	"tools.xdoubleu.com/apps/feeds/internal/models"
)

// LastChanceWindow is how long before expiry an item counts as "last chance".
const LastChanceWindow = 48 * time.Hour

// ExpireDue hides every unread item past its feed's lifetime and returns how
// many it hid.
func (s *FeedService) ExpireDue(ctx context.Context) (int64, error) {
	return s.items.ExpireDue(ctx)
}

// DigestItems returns up to limit of the newest unread items and of the
// last-chance items across all users, for the weekly digest.
func (s *FeedService) DigestItems(
	ctx context.Context,
	limit int,
) ([]models.DigestItem, []models.DigestItem, error) {
	newest, newestErr := s.items.DigestNewest(ctx, limit)
	lastChance, lastChanceErr := s.items.DigestLastChance(
		ctx, limit, LastChanceWindow,
	)
	if err := errors.Join(newestErr, lastChanceErr); err != nil {
		return nil, nil, err
	}
	return newest, lastChance, nil
}
