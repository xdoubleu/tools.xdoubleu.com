package services

import (
	"context"
	"errors"
	"net/http"
	"strings"

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
		ctx context.Context, userID string, titleID uuid.UUID, status string,
	) (uuid.UUID, error)
	SetStatus(ctx context.Context, userID string, id uuid.UUID, status string) error
	DeleteEntry(ctx context.Context, userID string, id uuid.UUID) error
	GetEntry(ctx context.Context, userID string, id uuid.UUID) (*models.Entry, error)
	ListEntries(
		ctx context.Context, userID string, f models.ListFilter,
	) ([]models.Entry, bool, error)
	StatusesByKey(
		ctx context.Context, userID string, keys []models.TitleKey,
	) (map[models.TitleKey]string, error)
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
// the user's backlog; re-adding only changes the status.
func (s *MovieService) Add(
	ctx context.Context,
	userID string,
	mediaType string,
	tmdbID int64,
	status string,
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

	titleID, err := s.repo.UpsertTitle(ctx, fromTMDB(*t))
	if err != nil {
		return nil, err
	}
	id, err := s.repo.AddEntry(ctx, userID, titleID, status)
	if err != nil {
		return nil, err
	}
	return s.repo.GetEntry(ctx, userID, id)
}

func (s *MovieService) SetStatus(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	status string,
) (*models.Entry, error) {
	if !models.IsStatus(status) {
		return nil, badRequest("unknown status")
	}
	if err := s.repo.SetStatus(ctx, userID, id, status); err != nil {
		return nil, err
	}
	return s.repo.GetEntry(ctx, userID, id)
}

func (s *MovieService) Remove(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return s.repo.DeleteEntry(ctx, userID, id)
}

func (s *MovieService) Get(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) (*models.Entry, error) {
	return s.repo.GetEntry(ctx, userID, id)
}

func (s *MovieService) List(
	ctx context.Context,
	userID string,
	f models.ListFilter,
) ([]models.Entry, bool, error) {
	if f.Status != "" && !models.IsStatus(f.Status) {
		return nil, false, badRequest("unknown status")
	}
	if f.MediaType != "" && !isMediaType(f.MediaType) {
		return nil, false, badRequest("media_type must be movie or series")
	}
	switch f.Sort {
	case "", models.SortAdded, models.SortTitle, models.SortRelease:
	default:
		return nil, false, badRequest("sort must be added, title or release")
	}
	return s.repo.ListEntries(ctx, userID, f)
}
