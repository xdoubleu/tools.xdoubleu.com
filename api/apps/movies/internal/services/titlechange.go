package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
)

// TitleChange is the validated edit of an entry's status, rating and latest
// watch date; zero fields change nothing.
type TitleChange struct {
	Status    string
	Rating    *int
	WatchedAt *time.Time
}

// SeasonTick ticks or unticks one series season.
type SeasonTick struct {
	Number  int
	Watched bool
}

// NewTitleChange validates the optional status, rating (0 clears) and
// watched_at (YYYY-MM-DD).
func NewTitleChange(status string, rating *int, watchedAt string) (TitleChange, error) {
	change := TitleChange{Status: status, Rating: nil, WatchedAt: nil}
	if status != "" && !models.IsStatus(status) {
		return change, badRequest("status must be want, watching, watched or dropped")
	}
	if rating != nil && (*rating < 0 || *rating > 5) {
		return change, badRequest("rating must be 0 to 5 (0 clears it)")
	}
	change.Rating = rating
	if watchedAt != "" {
		day, err := time.Parse(time.DateOnly, watchedAt)
		if err != nil {
			return change, badRequest("watched_at must be YYYY-MM-DD")
		}
		change.WatchedAt = &day
	}
	return change, nil
}

// Upsert returns the user's entry for a title, adding it as want when it is
// new, with the change applied. Adding a title already there creates no
// duplicate.
func (s *MovieService) Upsert(
	ctx context.Context,
	userID string,
	mediaType string,
	tmdbID int64,
	change TitleChange,
) (*models.Entry, error) {
	entry, _, err := s.Ensure(ctx, userID, mediaType, tmdbID)
	if err != nil {
		return nil, err
	}
	return s.applyChange(ctx, userID, entry, change)
}

// UpdateTitle ticks or unticks a season, then applies the change. A watch date
// given with a season tick dates that season's watch, not the entry's.
func (s *MovieService) UpdateTitle(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	change TitleChange,
	tick *SeasonTick,
) (*models.Entry, error) {
	entry, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if tick != nil {
		entry, err = s.tickSeason(ctx, userID, entry, *tick, change.WatchedAt)
		if err != nil {
			return nil, err
		}
		change.WatchedAt = nil
	}
	return s.applyChange(ctx, userID, entry, change)
}

// applyChange applies the change to the entry and returns it as stored.
// A status equal to the entry's is a no-op.
func (s *MovieService) applyChange(
	ctx context.Context,
	userID string,
	entry *models.Entry,
	change TitleChange,
) (*models.Entry, error) {
	if change.WatchedAt != nil && entry.Title.MediaType != tmdb.MediaTypeMovie {
		return nil, badRequest(
			"watched_at is for movies; for a series pass season and season_watched",
		)
	}
	if change.WatchedAt != nil && change.Status != models.StatusWatched &&
		(change.Status != "" || entry.Status != models.StatusWatched) {
		return nil, badRequest("watched_at needs status watched")
	}

	var err error
	if change.Status != "" && change.Status != entry.Status {
		if entry, err = s.SetStatus(
			ctx, userID, entry.ID, change.Status, false,
		); err != nil {
			return nil, err
		}
	}
	if change.Rating != nil {
		rating := change.Rating
		if *rating == 0 {
			rating = nil
		}
		if entry, err = s.SetRating(ctx, userID, entry.ID, rating); err != nil {
			return nil, err
		}
	}
	if change.WatchedAt != nil {
		return s.dateLatestWatch(ctx, userID, entry, nil, change.WatchedAt)
	}
	return entry, nil
}

// dateLatestWatch dates the entry's (or the season's) latest watch, adding a
// watch when there is none.
func (s *MovieService) dateLatestWatch(
	ctx context.Context,
	userID string,
	entry *models.Entry,
	season *int,
	day *time.Time,
) (*models.Entry, error) {
	watches := entry.WatchedAt
	if season != nil {
		watches = nil
		for _, se := range entry.Seasons {
			if se.Number == *season {
				watches = se.WatchedAt
			}
		}
	}
	if len(watches) == 0 {
		return s.AddWatchDate(ctx, userID, entry.ID, season, day)
	}
	return s.EditWatchDate(ctx, userID, entry.ID, season, len(watches)-1, day)
}

// tickSeason ticks or unticks a series season, dating the new watch when day
// is set.
func (s *MovieService) tickSeason(
	ctx context.Context,
	userID string,
	entry *models.Entry,
	tick SeasonTick,
	day *time.Time,
) (*models.Entry, error) {
	if entry.Title.MediaType != tmdb.MediaTypeSeries {
		return nil, badRequest("season only applies to series")
	}
	if day != nil && !tick.Watched {
		return nil, badRequest("watched_at needs season_watched true")
	}
	out, err := s.SetSeasonWatched(ctx, userID, entry.ID, tick.Number, tick.Watched, false)
	if err != nil || day == nil {
		return out, err
	}
	return s.dateLatestWatch(ctx, userID, out, &tick.Number, day)
}
