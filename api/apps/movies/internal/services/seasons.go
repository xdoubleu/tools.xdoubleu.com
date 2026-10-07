package services

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	"tools.xdoubleu.com/internal/database"
)

// loadSeasons fills a series' missing catalog seasons from TMDB. A TMDB
// failure leaves the entry without seasons rather than failing the read. A
// series already marked watched gets its aired seasons ticked, dated unknown,
// so its first season edit doesn't demote it.
func (s *MovieService) loadSeasons(
	ctx context.Context,
	userID string,
	entry *models.Entry,
) (*models.Entry, error) {
	if s.tmdb == nil {
		return entry, nil
	}
	t, err := s.tmdb.GetSeries(ctx, entry.Title.TMDBID)
	if err != nil {
		return entry, nil //nolint:nilerr // seasons are optional on a read
	}
	if _, err = s.storeTitle(ctx, *t); err != nil {
		return nil, err
	}
	if entry.Seasons, err = s.repo.ListSeasons(ctx, userID, entry.ID); err != nil {
		return nil, err
	}
	if entry.Status != models.StatusWatched {
		return entry, nil
	}
	return s.markSeriesWatched(ctx, userID, entry, true)
}

// watchAt is a new watch: now, or unknown.
func watchAt(unknownDate bool) *time.Time {
	if unknownDate {
		return nil
	}
	now := time.Now()
	return &now
}

// markSeriesWatched ticks every aired, unticked season of a series.
func (s *MovieService) markSeriesWatched(
	ctx context.Context,
	userID string,
	entry *models.Entry,
	unknownDate bool,
) (*models.Entry, error) {
	if entry.Title.MediaType != tmdb.MediaTypeSeries {
		return entry, nil
	}
	now := time.Now()
	changed := false
	for _, season := range entry.Seasons {
		if !season.Counts() || !season.Aired(now) || season.Ticked() {
			continue
		}
		err := s.repo.SetSeasonWatchedAt(
			ctx, userID, entry.ID, season.Number,
			[]*time.Time{watchAt(unknownDate)},
		)
		if err != nil {
			return nil, err
		}
		changed = true
	}
	if !changed {
		return entry, nil
	}
	return s.Get(ctx, userID, entry.ID)
}

// SetSeasonWatched ticks a season with one watch (now or unknown), or
// unticks it by clearing its watches, then re-derives the series status.
func (s *MovieService) SetSeasonWatched(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	number int,
	watched bool,
	unknownDate bool,
) (*models.Entry, error) {
	entry, season, err := s.season(ctx, userID, id, number)
	if err != nil {
		return nil, err
	}
	if watched == season.Ticked() {
		return entry, nil
	}
	var dates []*time.Time
	if watched {
		dates = []*time.Time{watchAt(unknownDate)}
	}
	return s.saveSeasonWatches(ctx, userID, entry, number, dates)
}

// AddWatchDate records another watch (a rewatch) of the entry, or of one of
// its seasons when season is set; a nil day is an unknown date.
func (s *MovieService) AddWatchDate(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	season *int,
	day *time.Time,
) (*models.Entry, error) {
	return s.editWatches(ctx, userID, id, season,
		func(dates []*time.Time) ([]*time.Time, error) {
			return append(dates, models.WatchDate(day)), nil
		})
}

// EditWatchDate changes the date of the index-th watch.
func (s *MovieService) EditWatchDate(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	season *int,
	index int,
	day *time.Time,
) (*models.Entry, error) {
	return s.editWatches(ctx, userID, id, season,
		func(dates []*time.Time) ([]*time.Time, error) {
			if index < 0 || index >= len(dates) {
				return nil, badRequest("no watch at that index")
			}
			dates[index] = models.WatchDate(day)
			return dates, nil
		})
}

// RemoveWatchDate deletes the index-th watch; removing a season's last watch
// unticks it.
func (s *MovieService) RemoveWatchDate(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	season *int,
	index int,
) (*models.Entry, error) {
	return s.editWatches(ctx, userID, id, season,
		func(dates []*time.Time) ([]*time.Time, error) {
			if index < 0 || index >= len(dates) {
				return nil, badRequest("no watch at that index")
			}
			return slices.Delete(dates, index, index+1), nil
		})
}

func (s *MovieService) editWatches(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	season *int,
	edit func([]*time.Time) ([]*time.Time, error),
) (*models.Entry, error) {
	if season != nil {
		entry, found, err := s.season(ctx, userID, id, *season)
		if err != nil {
			return nil, err
		}
		dates, err := edit(slices.Clone(found.WatchedAt))
		if err != nil {
			return nil, err
		}
		return s.saveSeasonWatches(ctx, userID, entry, *season, dates)
	}

	entry, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if entry.Title.MediaType == tmdb.MediaTypeSeries {
		return nil, badRequest("series watches are recorded per season")
	}
	dates, err := edit(slices.Clone(entry.WatchedAt))
	if err != nil {
		return nil, err
	}
	if err = s.repo.SetWatchedAt(ctx, userID, id, dates); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

// season loads a series entry and one of its seasons.
func (s *MovieService) season(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	number int,
) (*models.Entry, models.Season, error) {
	entry, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, models.Season{}, err
	}
	for _, season := range entry.Seasons {
		if season.Number == number {
			return entry, season, nil
		}
	}
	return nil, models.Season{}, database.ErrResourceNotFound
}

// saveSeasonWatches stores a season's watches and moves the series status to
// match its checklist; dropped series keep their status.
func (s *MovieService) saveSeasonWatches(
	ctx context.Context,
	userID string,
	entry *models.Entry,
	number int,
	dates []*time.Time,
) (*models.Entry, error) {
	err := s.repo.SetSeasonWatchedAt(ctx, userID, entry.ID, number, dates)
	if err != nil {
		return nil, err
	}
	updated, err := s.Get(ctx, userID, entry.ID)
	if err != nil || updated.Status == models.StatusDropped {
		return updated, err
	}
	next := models.SeriesStatus(updated.Seasons, time.Now())
	if next == updated.Status {
		return updated, nil
	}
	if err = s.repo.SetStatus(ctx, userID, entry.ID, next, false); err != nil {
		return nil, err
	}
	updated.Status = next
	return updated, nil
}
