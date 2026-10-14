package movies

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"tools.xdoubleu.com/apps/movies/internal/models"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/consumption"
)

const posterBaseURL = "https://image.tmdb.org/t/p/w185"

// FindTitle looks a recommended movie or series ("movie" or "series") up on
// TMDB, for the taste app; nil when TMDB has no title of that name. A match
// of year wins over the first match. Known is set when it is in userID's
// backlog, and OnMyServices when it isn't and streams on their services.
func (a *Movies) FindTitle(
	ctx context.Context,
	userID, mediaType, title string,
	year int,
) (*consumption.CatalogMatch, error) {
	results, err := a.services.Movies.Search(ctx, userID, title)
	if err != nil {
		return nil, err
	}
	best := pickTitle(results, mediaType, title, year)
	if best == nil {
		return nil, nil //nolint:nilnil // no match is not an error
	}
	t := best.Title
	path := "movie"
	if t.MediaType == "series" {
		path = "tv"
	}
	poster := ""
	if t.PosterPath != "" {
		poster = posterBaseURL + t.PosterPath
	}
	known := best.Status != nil
	onMyServices := !known && a.streamsOnMyServices(ctx, userID, t)
	return &consumption.CatalogMatch{
		Ref:          titleRef(t.MediaType, t.TMDBID),
		Title:        t.Title,
		Creator:      "",
		Year:         releaseYear(t),
		ImageURL:     poster,
		URL:          fmt.Sprintf("https://www.themoviedb.org/%s/%d", path, t.TMDBID),
		Known:        known,
		OnMyServices: onMyServices,
		Audience:     &best.VoteCount,
	}, nil
}

// streamsOnMyServices reports whether t streams on userID's services; a
// failed lookup reads as not, and is logged.
func (a *Movies) streamsOnMyServices(
	ctx context.Context,
	userID string,
	t models.Title,
) bool {
	on, err := a.services.Movies.StreamsOnMyServices(
		ctx, userID, models.TitleKey{MediaType: t.MediaType, TMDBID: t.TMDBID},
	)
	if err != nil {
		a.Logger.WarnContext(ctx, "streaming providers lookup failed",
			slog.Int64("tmdb_id", t.TMDBID), slog.Any("error", err))
	}
	return on
}

// WantTitle adds the title a FindTitle ref names to userID's backlog as want,
// unless it is already there; for the taste app.
func (a *Movies) WantTitle(ctx context.Context, userID, ref string) error {
	mediaType, id, _ := strings.Cut(ref, ":")
	tmdbID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return &iapp.HTTPError{Status: http.StatusBadRequest, Message: "invalid title ref"}
	}
	return a.services.Movies.Want(
		ctx, userID, models.TitleKey{MediaType: mediaType, TMDBID: tmdbID},
	)
}

// titleRef is a title's catalog ref: its media type and TMDB id.
func titleRef(mediaType string, tmdbID int64) string {
	return mediaType + ":" + strconv.FormatInt(tmdbID, 10)
}

// pickTitle prefers an exact title (or original title) of the media type,
// the one released in year first, else a match once a subtitle is dropped.
func pickTitle(
	results []models.SearchResult,
	mediaType, title string,
	year int,
) *models.SearchResult {
	var best *models.SearchResult
	var names []string
	var indices []int
	for i, r := range results {
		t := r.Title
		if t.MediaType != mediaType {
			continue
		}
		names, indices = append(names, t.Title), append(indices, i)
		exact := consumption.SameTitle(t.Title, title) ||
			consumption.SameTitle(t.OriginalTitle, title)
		if exact && (best == nil ||
			(releaseYear(t) == year && releaseYear(best.Title) != year)) {
			best = &results[i]
		}
	}
	if best == nil {
		if i := consumption.BestTitle(names, title); i >= 0 {
			best = &results[indices[i]]
		}
	}
	return best
}

func releaseYear(t models.Title) int {
	if t.ReleaseDate == nil {
		return 0
	}
	return t.ReleaseDate.Year()
}
