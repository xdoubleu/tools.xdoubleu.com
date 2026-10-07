package movies

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/apps/movies/internal/services"
	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
	"tools.xdoubleu.com/gen/movies/v1/moviesv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type moviesConnectHandler struct {
	app *Movies
}

var _ moviesv1connect.MoviesServiceHandler = (*moviesConnectHandler)(nil)

func userID(ctx context.Context) (string, error) {
	user := contexttools.GetValue[sharedmodels.User](ctx, constants.UserContextKey)
	if user == nil {
		return "", connect.NewError(
			connect.CodeUnauthenticated, errors.New("user not authenticated"),
		)
	}
	return user.ID, nil
}

func parseID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, connect.NewError(
			connect.CodeInvalidArgument, errors.New("invalid id"),
		)
	}
	return id, nil
}

func mapError(err error) error {
	if errors.Is(err, services.ErrNotConfigured) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connecttools.MapError(err)
}

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
	v := int32(*n) //nolint:gosec // runtimes and season counts fit in int32
	return &v
}

func protoWatches(watchedAt []*time.Time) []string {
	watched := make([]string, len(watchedAt))
	for i, w := range watchedAt {
		if w != nil {
			watched[i] = w.Format(time.RFC3339)
		}
	}
	return watched
}

func protoSeasons(seasons []models.Season) []*moviesv1.Season {
	now := time.Now()
	out := make([]*moviesv1.Season, len(seasons))
	for i, s := range seasons {
		out[i] = &moviesv1.Season{
			Number:       int32(s.Number),       //nolint:gosec // season numbers fit
			EpisodeCount: int32(s.EpisodeCount), //nolint:gosec // episode counts fit
			Name:         s.Name,
			AirDate:      formatDate(s.AirDate),
			WatchedAt:    protoWatches(s.WatchedAt),
			Aired:        s.Aired(now),
		}
	}
	return out
}

func protoEntry(e *models.Entry) *moviesv1.BacklogEntry {
	watched := protoWatches(e.WatchedAt)
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
		WatchedAt:     watched,
		AddedAt:       e.AddedAt.Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.Format(time.RFC3339),
	}
}

func protoSearchResult(r models.SearchResult) *moviesv1.SearchResult {
	return &moviesv1.SearchResult{
		MediaType:     r.Title.MediaType,
		TmdbId:        r.Title.TMDBID,
		Title:         r.Title.Title,
		OriginalTitle: r.Title.OriginalTitle,
		ReleaseDate:   formatDate(r.Title.ReleaseDate),
		PosterPath:    r.Title.PosterPath,
		Status:        r.Status,
	}
}
