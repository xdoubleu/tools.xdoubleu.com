package movies

import (
	"context"

	"connectrpc.com/connect"

	moviesv1 "tools.xdoubleu.com/gen/movies/v1"
)

func (h *moviesConnectHandler) GetStats(
	ctx context.Context,
	_ *connect.Request[moviesv1.GetStatsRequest],
) (*connect.Response[moviesv1.GetStatsResponse], error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	stats, err := h.app.services.Movies.Stats(ctx, uid)
	if err != nil {
		return nil, mapError(err)
	}

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
	return connect.NewResponse(&moviesv1.GetStatsResponse{
		Months:         months,
		Genres:         genres,
		Ratings:        ratings,
		MoviesWatched:  count32(stats.MoviesWatched),
		SeriesWatched:  count32(stats.SeriesWatched),
		SeasonsWatched: count32(stats.SeasonsWatched),
	}), nil
}

func count32(n int) int32 {
	return int32(n) //nolint:gosec // a user's title counts fit in int32
}
