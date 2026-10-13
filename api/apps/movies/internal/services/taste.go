package services

import (
	"context"
	"slices"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
)

// Want adds the title to userID's backlog as want, unless it is already there.
func (s *MovieService) Want(
	ctx context.Context,
	userID string,
	k models.TitleKey,
) error {
	statuses, err := s.repo.StatusesByKey(ctx, userID, []models.TitleKey{k})
	if err != nil {
		return err
	}
	if _, ok := statuses[k]; ok {
		return nil
	}
	_, err = s.Add(ctx, userID, k.MediaType, k.TMDBID, models.StatusWant, false)
	return err
}

// StreamsOnMyServices reports whether the title streams on one of userID's
// services.
func (s *MovieService) StreamsOnMyServices(
	ctx context.Context,
	userID string,
	k models.TitleKey,
) (bool, error) {
	mine, err := s.repo.GetProviderIDs(ctx, userID)
	if err != nil || len(mine) == 0 {
		return false, err
	}
	if s.tmdb == nil {
		return false, ErrNotConfigured
	}
	t, err := s.fetch(ctx, k)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(t.Providers, func(p tmdb.Provider) bool {
		return models.StreamsOnMyServices(p.OfferType) && slices.Contains(mine, p.ID)
	}), nil
}

// SharedSubjects returns the owner key of every shared list with a title.
func (s *MovieService) SharedSubjects(ctx context.Context) ([]string, error) {
	return s.repo.SharedOwners(ctx)
}
