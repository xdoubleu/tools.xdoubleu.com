package services

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

const coverWarmConcurrency = 4

// WarmUserCovers caches the covers of userID's books, coverWarmConcurrency at
// a time. A call while that user's warm is running joins it rather than
// starting another. A missing cover or failed fetch is logged and skipped.
func (s *BookService) WarmUserCovers(
	ctx context.Context,
	userID string,
	bookIDs []uuid.UUID,
) {
	_, _, _ = s.coverWarms.Do(userID, func() (any, error) {
		s.warmUserCovers(ctx, userID, bookIDs)
		return struct{}{}, nil
	})
}

func (s *BookService) warmUserCovers(
	ctx context.Context,
	userID string,
	bookIDs []uuid.UUID,
) {
	ids := make(chan uuid.UUID)
	var wg sync.WaitGroup
	for range coverWarmConcurrency {
		wg.Go(func() {
			for id := range ids {
				if _, err := s.EnsureUserCoverCached(ctx, userID, id); err != nil {
					s.logger.Warn("failed to warm kobo cover",
						"bookID", id, "err", err)
				}
			}
		})
	}
	for _, id := range bookIDs {
		ids <- id
	}
	close(ids)
	wg.Wait()
}
