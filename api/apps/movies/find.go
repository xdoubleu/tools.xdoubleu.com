package movies

import (
	"context"
	"fmt"
	"strconv"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/internal/consumption"
)

const posterBaseURL = "https://image.tmdb.org/t/p/w185"

// FindTitle looks a recommended movie or series ("movie" or "series") up on
// TMDB, for the taste app; nil when TMDB has no title of that name. A match
// of year wins over the first match. Known is set when it is in userID's
// backlog.
func (a *Movies) FindTitle(
	ctx context.Context,
	userID, mediaType, title string,
	year int,
) (*consumption.CatalogMatch, error) {
	results, err := a.services.Movies.Search(ctx, userID, title)
	if err != nil {
		return nil, err
	}
	var best *models.SearchResult
	for i, r := range results {
		t := r.Title
		named := consumption.SameTitle(t.Title, title) ||
			consumption.SameTitle(t.OriginalTitle, title)
		if t.MediaType != mediaType || !named {
			continue
		}
		if best == nil || (releaseYear(t) == year && releaseYear(best.Title) != year) {
			best = &results[i]
		}
	}
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
	return &consumption.CatalogMatch{
		Ref:      t.MediaType + ":" + strconv.FormatInt(t.TMDBID, 10),
		Title:    t.Title,
		Creator:  "",
		Year:     releaseYear(t),
		ImageURL: poster,
		URL:      fmt.Sprintf("https://www.themoviedb.org/%s/%d", path, t.TMDBID),
		Known:    best.Status != nil,
	}, nil
}

func releaseYear(t models.Title) int {
	if t.ReleaseDate == nil {
		return 0
	}
	return t.ReleaseDate.Year()
}
