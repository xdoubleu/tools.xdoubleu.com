package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/podcasts/internal/models"
	"tools.xdoubleu.com/apps/podcasts/internal/repositories"
	"tools.xdoubleu.com/apps/podcasts/pkg/itunes"
	"tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/auth"
	"tools.xdoubleu.com/internal/database"
)

// ErrUpstream is returned when iTunes can't be reached or answers badly.
var ErrUpstream = errors.New("iTunes is unavailable")

// minQueryLen keeps one-letter searches from hitting iTunes.
const minQueryLen = 2

type Services struct {
	Auth  auth.Service
	Shows *ShowService
}

func New(
	repos *repositories.Repositories,
	authService auth.Service,
	c itunes.Client,
) *Services {
	return &Services{
		Auth:  authService,
		Shows: &ShowService{repo: repos.Shows, itunes: c},
	}
}

type ShowService struct {
	repo   *repositories.ShowsRepository
	itunes itunes.Client
}

// SearchResult is an iTunes show and whether the user already favourited it.
type SearchResult struct {
	itunes.Show
	Favourite bool
}

func badRequest(msg string) error {
	return &app.HTTPError{Status: http.StatusBadRequest, Message: msg}
}

// Search finds shows by name; a query under two characters finds none.
func (s *ShowService) Search(
	ctx context.Context,
	userID string,
	query string,
) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < minQueryLen {
		return []SearchResult{}, nil
	}
	shows, err := s.itunes.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUpstream, err)
	}

	ids := make([]int64, len(shows))
	for i, show := range shows {
		ids[i] = show.ID
	}
	favourited, err := s.repo.Favourited(ctx, userID, ids)
	if err != nil {
		return nil, err
	}

	out := make([]SearchResult, len(shows))
	for i, show := range shows {
		out[i] = SearchResult{Show: show, Favourite: favourited[show.ID]}
	}
	return out, nil
}

// List returns the user's favourites, newest first.
func (s *ShowService) List(ctx context.Context, userID string) ([]models.Show, error) {
	return s.repo.List(ctx, userID)
}

// Add favourites a show. Its feed URL comes from an iTunes lookup, never
// from the caller.
func (s *ShowService) Add(
	ctx context.Context,
	userID string,
	itunesID int64,
) (*models.Show, error) {
	if itunesID <= 0 {
		return nil, badRequest("invalid iTunes id")
	}
	show, err := s.itunes.Lookup(ctx, itunesID)
	switch {
	case errors.Is(err, itunes.ErrNotFound):
		return nil, database.ErrResourceNotFound
	case err != nil:
		return nil, fmt.Errorf("%w: %w", ErrUpstream, err)
	}
	return s.repo.Add(ctx, models.Show{
		ID:         uuid.Nil,
		UserID:     userID,
		ITunesID:   show.ID,
		Title:      show.Title,
		Author:     show.Author,
		ArtworkURL: show.ArtworkURL,
		FeedURL:    show.FeedURL,
		AppleURL:   show.AppleURL,
		AddedAt:    time.Time{},
	})
}

// Remove unfavourites a show.
func (s *ShowService) Remove(ctx context.Context, userID string, id uuid.UUID) error {
	return s.repo.Remove(ctx, userID, id)
}
