package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/database"
)

// ErrNotConfigured is returned by TMDB-backed calls when no API key is set.
var ErrNotConfigured = errors.New("TMDB is not configured")

// moviesStore is the storage surface MovieService needs.
type moviesStore interface {
	UpsertTitle(ctx context.Context, t models.Title) (uuid.UUID, error)
	AddEntry(
		ctx context.Context,
		userID string,
		titleID uuid.UUID,
		status string,
		unknownDate bool,
	) (uuid.UUID, error)
	SetStatus(
		ctx context.Context,
		userID string,
		id uuid.UUID,
		status string,
		unknownDate bool,
	) error
	SetRating(ctx context.Context, userID string, id uuid.UUID, rating *int) error
	DeleteEntry(ctx context.Context, userID string, id uuid.UUID) error
	GetEntry(ctx context.Context, userID string, id uuid.UUID) (*models.Entry, error)
	ListEntries(
		ctx context.Context, userID string, f models.ListFilter,
	) ([]models.Entry, bool, error)
	StatusesByKey(
		ctx context.Context, userID string, keys []models.TitleKey,
	) (map[models.TitleKey]string, error)
	UpsertSeasons(ctx context.Context, titleID uuid.UUID, seasons []models.Season) error
	ListSeasons(
		ctx context.Context, userID string, entryID uuid.UUID,
	) ([]models.Season, error)
	SetSeasonWatchedAt(
		ctx context.Context,
		userID string,
		entryID uuid.UUID,
		number int,
		watchedAt []*time.Time,
	) error
	SetWatchedAt(
		ctx context.Context, userID string, entryID uuid.UUID, watchedAt []*time.Time,
	) error
}

type MovieService struct {
	repo moviesStore
	tmdb tmdb.Client
}

func badRequest(msg string) error {
	return &app.HTTPError{Status: http.StatusBadRequest, Message: msg}
}

func isMediaType(s string) bool {
	return s == tmdb.MediaTypeMovie || s == tmdb.MediaTypeSeries
}

func fromTMDB(t tmdb.Title) models.Title {
	return models.Title{
		ID:            uuid.Nil,
		MediaType:     t.MediaType,
		TMDBID:        t.TMDBID,
		Title:         t.Title,
		OriginalTitle: t.OriginalTitle,
		ReleaseDate:   t.ReleaseDate,
		PosterPath:    t.PosterPath,
		Overview:      t.Overview,
		Genres:        t.Genres,
		Runtime:       t.Runtime,
		SeasonCount:   t.SeasonCount,
	}
}

func (s *MovieService) Search(
	ctx context.Context,
	userID string,
	query string,
) ([]models.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if s.tmdb == nil {
		return nil, ErrNotConfigured
	}

	titles, err := s.tmdb.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	keys := make([]models.TitleKey, len(titles))
	for i, t := range titles {
		keys[i] = models.TitleKey{MediaType: t.MediaType, TMDBID: t.TMDBID}
	}
	statuses, err := s.repo.StatusesByKey(ctx, userID, keys)
	if err != nil {
		return nil, err
	}

	results := make([]models.SearchResult, len(titles))
	for i, t := range titles {
		results[i] = models.SearchResult{Title: fromTMDB(t), Status: nil}
		if status, ok := statuses[keys[i]]; ok {
			results[i].Status = &status
		}
	}
	return results, nil
}

// Add fetches the title's details from TMDB into the catalog and puts it in
// the user's backlog; re-adding only changes the status. A series added as
// watched ticks its aired seasons; unknownDate dates those watches unknown.
func (s *MovieService) Add(
	ctx context.Context,
	userID string,
	mediaType string,
	tmdbID int64,
	status string,
	unknownDate bool,
) (*models.Entry, error) {
	if !isMediaType(mediaType) {
		return nil, badRequest("media_type must be movie or series")
	}
	if status != models.StatusWant && status != models.StatusWatched {
		return nil, badRequest("status must be want or watched")
	}
	if s.tmdb == nil {
		return nil, ErrNotConfigured
	}

	var t *tmdb.Title
	var err error
	if mediaType == tmdb.MediaTypeMovie {
		t, err = s.tmdb.GetMovie(ctx, tmdbID)
	} else {
		t, err = s.tmdb.GetSeries(ctx, tmdbID)
	}
	if errors.Is(err, tmdb.ErrNotFound) {
		return nil, database.ErrResourceNotFound
	}
	if err != nil {
		return nil, err
	}

	titleID, err := s.storeTitle(ctx, *t)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.AddEntry(ctx, userID, titleID, status, unknownDate)
	if err != nil {
		return nil, err
	}
	entry, err := s.Get(ctx, userID, id)
	if err != nil || status != models.StatusWatched {
		return entry, err
	}
	return s.markSeriesWatched(ctx, userID, entry, unknownDate)
}

func (s *MovieService) storeTitle(
	ctx context.Context,
	t tmdb.Title,
) (uuid.UUID, error) {
	titleID, err := s.repo.UpsertTitle(ctx, fromTMDB(t))
	if err != nil || len(t.Seasons) == 0 {
		return titleID, err
	}
	seasons := make([]models.Season, len(t.Seasons))
	for i, ts := range t.Seasons {
		//nolint:exhaustruct // catalog fields only; watches are per user
		seasons[i] = models.Season{
			Number:       ts.Number,
			Name:         ts.Name,
			AirDate:      ts.AirDate,
			EpisodeCount: ts.EpisodeCount,
		}
	}
	return titleID, s.repo.UpsertSeasons(ctx, titleID, seasons)
}

// SetStatus changes the entry's status. Setting a series to watched ticks
// its aired seasons; unknownDate dates new watches unknown.
func (s *MovieService) SetStatus(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	status string,
	unknownDate bool,
) (*models.Entry, error) {
	if !models.IsStatus(status) {
		return nil, badRequest("unknown status")
	}
	if err := s.repo.SetStatus(ctx, userID, id, status, unknownDate); err != nil {
		return nil, err
	}
	entry, err := s.Get(ctx, userID, id)
	if err != nil || status != models.StatusWatched {
		return entry, err
	}
	return s.markSeriesWatched(ctx, userID, entry, unknownDate)
}

// SetRating rates the entry 1-5 stars; nil clears the rating.
func (s *MovieService) SetRating(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	rating *int,
) (*models.Entry, error) {
	if !models.IsRating(rating) {
		return nil, badRequest("rating must be 1 to 5")
	}
	if err := s.repo.SetRating(ctx, userID, id, rating); err != nil {
		return nil, err
	}
	return s.Get(ctx, userID, id)
}

func (s *MovieService) Remove(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return s.repo.DeleteEntry(ctx, userID, id)
}

// Get returns the entry with its seasons, loading a series' seasons from
// TMDB the first time they are missing.
func (s *MovieService) Get(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) (*models.Entry, error) {
	entry, err := s.repo.GetEntry(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if entry.Title.MediaType != tmdb.MediaTypeSeries {
		return entry, nil
	}
	if entry.Seasons, err = s.repo.ListSeasons(ctx, userID, id); err != nil {
		return nil, err
	}
	if len(entry.Seasons) == 0 {
		return s.loadSeasons(ctx, userID, entry)
	}
	return entry, nil
}

func (s *MovieService) List(
	ctx context.Context,
	userID string,
	f models.ListFilter,
) ([]models.Entry, bool, error) {
	if f.Offset < 0 {
		return nil, false, badRequest("offset must not be negative")
	}
	if f.Status != "" && !models.IsStatus(f.Status) {
		return nil, false, badRequest("unknown status")
	}
	if f.MediaType != "" && !isMediaType(f.MediaType) {
		return nil, false, badRequest("media_type must be movie or series")
	}
	switch f.Sort {
	case "", models.SortAdded, models.SortTitle, models.SortRelease,
		models.SortRating:
	default:
		return nil, false, badRequest("sort must be added, title, release or rating")
	}
	return s.repo.ListEntries(ctx, userID, f)
}
