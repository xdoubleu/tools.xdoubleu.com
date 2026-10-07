package services

import (
	"context"
	"errors"
	"log/slog"

	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
)

// RefreshCatalog deletes titles no backlog holds, then re-fetches the due
// ones from TMDB. A failing title is logged and skipped so the rest refresh.
func (s *MovieService) RefreshCatalog(ctx context.Context, logger *slog.Logger) error {
	deleted, err := s.repo.DeleteOrphanTitles(ctx)
	if err != nil {
		return err
	}
	logger.DebugContext(ctx, "deleted orphan titles", slog.Int64("count", deleted))
	if s.tmdb == nil {
		return nil
	}

	due, err := s.repo.DueTitles(ctx)
	if err != nil {
		return err
	}
	for _, k := range due {
		attrs := []any{slog.String("mediaType", k.MediaType), slog.Int64("tmdbID", k.TMDBID)}
		t, fetchErr := s.fetch(ctx, k)
		if errors.Is(fetchErr, tmdb.ErrNotFound) {
			logger.WarnContext(ctx, "title gone from TMDB", attrs...)
			continue
		}
		if fetchErr == nil {
			_, fetchErr = s.storeTitle(ctx, *t)
		}
		if fetchErr != nil {
			logger.ErrorContext(ctx, "refreshing title failed",
				append(attrs, slog.Any("error", fetchErr))...)
		}
	}
	return nil
}
