package tmdb

import (
	"errors"
	"time"
)

// ErrNotFound is returned when TMDB has no title with the requested ID.
var ErrNotFound = errors.New("tmdb: title not found")

const (
	MediaTypeMovie  = "movie"
	MediaTypeSeries = "series"
)

// Title is a movie or series. ReleaseDate is the first air date for series,
// nil when TMDB has none. Overview, Genres, Runtime, SeasonCount and Seasons
// are only set by GetMovie/GetSeries.
type Title struct {
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
	Seasons       []Season
}

// Season is one season of a series; Number 0 is TMDB's "Specials".
type Season struct {
	Number       int
	Name         string
	AirDate      *time.Time
	EpisodeCount int
}

type searchResponse struct {
	Results []searchResult `json:"results"`
}

type searchResult struct {
	MediaType     string `json:"media_type"`
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Name          string `json:"name"`
	OriginalTitle string `json:"original_title"`
	OriginalName  string `json:"original_name"`
	ReleaseDate   string `json:"release_date"`
	FirstAirDate  string `json:"first_air_date"`
	PosterPath    string `json:"poster_path"`
}

type genre struct {
	Name string `json:"name"`
}

type movieResponse struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	Overview      string  `json:"overview"`
	Genres        []genre `json:"genres"`
	Runtime       int     `json:"runtime"`
}

type seriesResponse struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	OriginalName    string  `json:"original_name"`
	FirstAirDate    string  `json:"first_air_date"`
	PosterPath      string  `json:"poster_path"`
	Overview        string  `json:"overview"`
	Genres          []genre `json:"genres"`
	NumberOfSeasons int     `json:"number_of_seasons"`
	Seasons         []struct {
		SeasonNumber int    `json:"season_number"`
		Name         string `json:"name"`
		AirDate      string `json:"air_date"`
		EpisodeCount int    `json:"episode_count"`
	} `json:"seasons"`
}

func parseDate(s string) *time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil
	}
	return &d
}

func genreNames(gs []genre) []string {
	names := make([]string, 0, len(gs))
	for _, g := range gs {
		if g.Name != "" {
			names = append(names, g.Name)
		}
	}
	return names
}

func positive(n int) *int {
	if n <= 0 {
		return nil
	}
	return &n
}

func (r searchResult) toTitle() (Title, bool) {
	//nolint:exhaustruct // detail-only fields stay empty for search results
	t := Title{TMDBID: r.ID, PosterPath: r.PosterPath}
	switch r.MediaType {
	case "movie":
		t.MediaType = MediaTypeMovie
		t.Title, t.OriginalTitle = r.Title, r.OriginalTitle
		t.ReleaseDate = parseDate(r.ReleaseDate)
	case "tv":
		t.MediaType = MediaTypeSeries
		t.Title, t.OriginalTitle = r.Name, r.OriginalName
		t.ReleaseDate = parseDate(r.FirstAirDate)
	default:
		return t, false
	}
	return t, true
}

func (r movieResponse) toTitle() Title {
	return Title{
		MediaType:     MediaTypeMovie,
		TMDBID:        r.ID,
		Title:         r.Title,
		OriginalTitle: r.OriginalTitle,
		ReleaseDate:   parseDate(r.ReleaseDate),
		PosterPath:    r.PosterPath,
		Overview:      r.Overview,
		Genres:        genreNames(r.Genres),
		Runtime:       positive(r.Runtime),
		SeasonCount:   nil,
		Seasons:       nil,
	}
}

func (r seriesResponse) toTitle() Title {
	seasons := make([]Season, len(r.Seasons))
	for i, s := range r.Seasons {
		seasons[i] = Season{
			Number:       s.SeasonNumber,
			Name:         s.Name,
			AirDate:      parseDate(s.AirDate),
			EpisodeCount: s.EpisodeCount,
		}
	}
	return Title{
		MediaType:     MediaTypeSeries,
		TMDBID:        r.ID,
		Title:         r.Name,
		OriginalTitle: r.OriginalName,
		ReleaseDate:   parseDate(r.FirstAirDate),
		PosterPath:    r.PosterPath,
		Overview:      r.Overview,
		Genres:        genreNames(r.Genres),
		Runtime:       nil,
		SeasonCount:   positive(r.NumberOfSeasons),
		Seasons:       seasons,
	}
}
