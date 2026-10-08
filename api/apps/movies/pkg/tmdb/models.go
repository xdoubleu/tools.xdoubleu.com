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

// Offer types of a Provider.
const (
	OfferFlatrate = "flatrate"
	OfferFree     = "free"
	OfferAds      = "ads"
	OfferRent     = "rent"
	OfferBuy      = "buy"
)

// watchRegion is the only region providers are kept for.
const watchRegion = "BE"

// justWatchProviderID is TMDB's "JustWatch TV" pseudo-provider.
const justWatchProviderID = 2285

// Provider is one way to watch a title in Belgium.
type Provider struct {
	ID              int64
	Name            string
	LogoPath        string
	OfferType       string
	DisplayPriority int
}

// Title is a movie or series. ReleaseDate is the first air date for series,
// nil when TMDB has none. Overview, Genres, Runtime, SeasonCount, Seasons,
// WatchLink and Providers are only set by GetMovie/GetSeries.
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
	// WatchLink is TMDB's watch page for Belgium, not a streaming service.
	WatchLink string
	Providers []Provider
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

// RegionProvider is a provider TMDB lists for Belgium, whatever it offers.
type RegionProvider struct {
	ID       int64
	Name     string
	LogoPath string
}

type regionProviderList struct {
	Results []struct {
		ProviderID      int64          `json:"provider_id"`
		ProviderName    string         `json:"provider_name"`
		LogoPath        string         `json:"logo_path"`
		DisplayPriority map[string]int `json:"display_priorities"`
	} `json:"results"`
}

// merge adds the list's Belgian providers to into, keeping the best
// (lowest) display priority per provider.
func (l regionProviderList) merge(into map[int64]priorityProvider) {
	for _, r := range l.Results {
		prio, ok := r.DisplayPriority[watchRegion]
		if !ok || r.ProviderID == justWatchProviderID {
			continue
		}
		if cur, seen := into[r.ProviderID]; seen && cur.priority <= prio {
			continue
		}
		into[r.ProviderID] = priorityProvider{
			RegionProvider: RegionProvider{
				ID: r.ProviderID, Name: r.ProviderName, LogoPath: r.LogoPath,
			},
			priority: prio,
		}
	}
}

type priorityProvider struct {
	RegionProvider
	priority int
}

type providerEntry struct {
	ProviderID      int64  `json:"provider_id"`
	ProviderName    string `json:"provider_name"`
	LogoPath        string `json:"logo_path"`
	DisplayPriority int    `json:"display_priority"`
}

// regionProviders is one region's offers; TMDB omits the empty keys.
type regionProviders struct {
	Link     string          `json:"link"`
	Flatrate []providerEntry `json:"flatrate"`
	Free     []providerEntry `json:"free"`
	Ads      []providerEntry `json:"ads"`
	Rent     []providerEntry `json:"rent"`
	Buy      []providerEntry `json:"buy"`
}

type watchProviders struct {
	Results map[string]regionProviders `json:"results"`
}

// toProviders returns the Belgian watch link and offers, without the
// JustWatch pseudo-provider.
func (w watchProviders) toProviders() (string, []Provider) {
	r := w.Results[watchRegion]
	var out []Provider
	for _, group := range []struct {
		offer   string
		entries []providerEntry
	}{
		{OfferFlatrate, r.Flatrate},
		{OfferFree, r.Free},
		{OfferAds, r.Ads},
		{OfferRent, r.Rent},
		{OfferBuy, r.Buy},
	} {
		for _, e := range group.entries {
			if e.ProviderID == justWatchProviderID {
				continue
			}
			out = append(out, Provider{
				ID:              e.ProviderID,
				Name:            e.ProviderName,
				LogoPath:        e.LogoPath,
				OfferType:       group.offer,
				DisplayPriority: e.DisplayPriority,
			})
		}
	}
	return r.Link, out
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

	WatchProviders watchProviders `json:"watch/providers"`
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

	WatchProviders watchProviders `json:"watch/providers"`
	Seasons        []struct {
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
	link, providers := r.WatchProviders.toProviders()
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
		WatchLink:     link,
		Providers:     providers,
	}
}

func (r seriesResponse) toTitle() Title {
	link, providers := r.WatchProviders.toProviders()
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
		WatchLink:     link,
		Providers:     providers,
	}
}
