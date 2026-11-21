// Package protomap maps the movies domain models to their wire messages, for
// the Connect and MCP adapters.
package protomap

import (
	"time"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/pkg/tmdb"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

func optionalInt32(n *int) *int32 {
	if n == nil {
		return nil
	}
	v := int32(*n) //nolint:gosec // small counts and ratings fit in int32
	return &v
}

func watches(watchedAt []*time.Time) []string {
	watched := make([]string, len(watchedAt))
	for i, w := range watchedAt {
		if w != nil {
			watched[i] = w.Format(time.RFC3339)
		}
	}
	return watched
}

// Seasons maps seasons the service loaded, so Aired is as of its clock.
func Seasons(seasons []models.Season) []*moviesv1.Season {
	out := make([]*moviesv1.Season, len(seasons))
	for i, s := range seasons {
		out[i] = &moviesv1.Season{
			Number:       int32(s.Number),       //nolint:gosec // season numbers fit
			EpisodeCount: int32(s.EpisodeCount), //nolint:gosec // episode counts fit
			Name:         s.Name,
			AirDate:      formatDate(s.AirDate),
			WatchedAt:    watches(s.WatchedAt),
			Aired:        s.HasAired,
		}
	}
	return out
}

func Offers(providers []models.Provider) []*moviesv1.ProviderOffers {
	var out []*moviesv1.ProviderOffers
	for _, p := range providers {
		if n := len(out); n == 0 || out[n-1].OfferType != p.OfferType {
			out = append(out, &moviesv1.ProviderOffers{OfferType: p.OfferType})
		}
		last := out[len(out)-1]
		last.Providers = append(last.Providers, &moviesv1.Provider{
			Id: p.ID, Name: p.Name, LogoPath: p.LogoPath, Mine: p.Mine,
		})
	}
	return out
}

func Entry(e *models.Entry) *moviesv1.BacklogEntry {
	return &moviesv1.BacklogEntry{
		Id:            e.ID.String(),
		MediaType:     e.Title.MediaType,
		TmdbId:        e.Title.TMDBID,
		Title:         e.Title.Title,
		OriginalTitle: e.Title.OriginalTitle,
		ReleaseDate:   formatDate(e.Title.ReleaseDate),
		PosterPath:    e.Title.PosterPath,
		Genres:        e.Title.Genres,
		Runtime:       optionalInt32(e.Title.Runtime),
		SeasonCount:   optionalInt32(e.Title.SeasonCount),
		Status:        e.Status,
		WatchedAt:     watches(e.WatchedAt),
		AddedAt:       e.AddedAt.Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.Format(time.RFC3339),
		Rating:        optionalInt32(e.Rating),
		HasNewSeason:  e.HasNewSeason,
		OnMyServices:  e.OnMyServices,
	}
}

func Entries(entries []models.Entry) []*moviesv1.BacklogEntry {
	out := make([]*moviesv1.BacklogEntry, len(entries))
	for i := range entries {
		out[i] = Entry(&entries[i])
	}
	return out
}

func SearchResults(results []models.SearchResult) []*moviesv1.SearchResult {
	out := make([]*moviesv1.SearchResult, len(results))
	for i, r := range results {
		out[i] = &moviesv1.SearchResult{
			MediaType:     r.Title.MediaType,
			TmdbId:        r.Title.TMDBID,
			Title:         r.Title.Title,
			OriginalTitle: r.Title.OriginalTitle,
			ReleaseDate:   formatDate(r.Title.ReleaseDate),
			PosterPath:    r.Title.PosterPath,
			Status:        r.Status,
		}
	}
	return out
}

func count32(n int) int32 {
	return int32(n) //nolint:gosec // a user's title counts fit in int32
}

func Stats(stats *models.Stats) *moviesv1.GetStatsResponse {
	months := make([]*moviesv1.MonthWatches, len(stats.Months))
	for i, m := range stats.Months {
		months[i] = &moviesv1.MonthWatches{
			Month: m.Month, Movies: count32(m.Movies), Seasons: count32(m.Seasons),
		}
	}
	genres := make([]*moviesv1.GenreCount, len(stats.Genres))
	for i, g := range stats.Genres {
		genres[i] = &moviesv1.GenreCount{Genre: g.Genre, Count: count32(g.Count)}
	}
	ratings := make([]int32, len(stats.Ratings))
	for i, n := range stats.Ratings {
		ratings[i] = count32(n)
	}
	return &moviesv1.GetStatsResponse{
		Months:         months,
		Genres:         genres,
		Ratings:        ratings,
		MoviesWatched:  count32(stats.MoviesWatched),
		SeriesWatched:  count32(stats.SeriesWatched),
		SeasonsWatched: count32(stats.SeasonsWatched),
	}
}

// Providers maps the streaming providers a user can pick from.
func Providers(list []tmdb.RegionProvider) []*moviesv1.Provider {
	out := make([]*moviesv1.Provider, len(list))
	for i, p := range list {
		out[i] = &moviesv1.Provider{Id: p.ID, Name: p.Name, LogoPath: p.LogoPath}
	}
	return out
}
