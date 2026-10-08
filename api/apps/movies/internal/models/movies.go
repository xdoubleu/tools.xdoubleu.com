package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusWant     = "want"
	StatusWatching = "watching"
	StatusWatched  = "watched"
	StatusDropped  = "dropped"

	SortAdded   = "added"
	SortTitle   = "title"
	SortRelease = "release"
	SortRating  = "rating"
)

// Title is a row of the shared TMDB catalog.
type Title struct {
	ID            uuid.UUID
	MediaType     string
	TMDBID        int64
	Title         string
	OriginalTitle string
	ReleaseDate   *time.Time
	PosterPath    string
	Overview      string
	Genres        []string
	Runtime       *int
	SeasonCount   *int
	// WatchLink is TMDB's watch page for Belgium; only loaded for a single
	// entry.
	WatchLink string
	// ProvidersFetched is false for titles stored before providers were
	// fetched; only loaded for a single entry.
	ProvidersFetched bool
}

// Provider is one way to watch a title in Belgium. OfferType is flatrate,
// free, ads, rent or buy.
type Provider struct {
	ID              int64
	Name            string
	LogoPath        string
	OfferType       string
	DisplayPriority int
	// Mine is set on a flatrate, free or ads offer from one of the user's
	// services.
	Mine bool
}

// StreamsOnMyServices reports whether the offer type counts for the "on my
// services" filter: renting and buying don't.
func StreamsOnMyServices(offerType string) bool {
	return offerType == "flatrate" || offerType == "free" || offerType == "ads"
}

// Entry is a title in one user's backlog. A nil WatchedAt element is a watch
// with an unknown date; series keep their watches per season instead. Seasons
// is only loaded for a single entry.
type Entry struct {
	ID     uuid.UUID
	UserID string
	Title  Title
	Status string
	// HasNewSeason is set on a watched series with an aired, unticked
	// season.
	HasNewSeason bool
	// OnMyServices is set when a flatrate, free or ads provider of the title
	// is one of the user's services.
	OnMyServices bool
	// Rating is 1-5 stars, nil when unrated; any status can keep one.
	Rating    *int
	WatchedAt []*time.Time
	AddedAt   time.Time
	UpdatedAt time.Time
	Seasons   []Season
	// Providers is only loaded for a single entry.
	Providers []Provider
}

// ListFilter narrows a backlog listing; empty fields match everything.
type ListFilter struct {
	Status    string
	MediaType string
	Sort      string
	// NewSeason keeps only entries with HasNewSeason.
	NewSeason bool
	// OnMyServices keeps only entries with OnMyServices.
	OnMyServices bool
	Limit        int32
	Offset       int32
}

// SearchResult is a TMDB match; Status is set when it is already in the
// caller's backlog.
type SearchResult struct {
	Title  Title
	Status *string
}

// TitleKey identifies a catalog title by its TMDB identity.
type TitleKey struct {
	MediaType string
	TMDBID    int64
}

func IsStatus(s string) bool {
	switch s {
	case StatusWant, StatusWatching, StatusWatched, StatusDropped:
		return true
	}
	return false
}

// IsRating reports whether r is a valid rating; nil (unrated) is.
func IsRating(r *int) bool {
	return r == nil || (*r >= 1 && *r <= 5)
}
