package repositories

import (
	"context"

	"tools.xdoubleu.com/apps/movies/internal/models"
)

// statsTimezone buckets watches into the owner's months.
const statsTimezone = "Europe/Brussels"

// seasonWatches are the user's watches of counted seasons TMDB still lists.
const seasonWatches = `
	SELECT ut.id AS user_title_id, us.watched_at
	FROM movies.user_titles ut
	JOIN movies.user_seasons us ON us.user_title_id = ut.id
	JOIN movies.seasons s
		ON s.title_id = ut.title_id AND s.season_number = us.season_number
	WHERE ut.user_id = $1 AND us.season_number > 0
		AND cardinality(us.watched_at) > 0`

// watchedTitles are the user's titles with any watch.
const watchedTitles = `
	SELECT ut.id, ut.status, t.media_type, t.genres
	FROM movies.user_titles ut
	JOIN movies.titles t ON t.id = ut.title_id
	WHERE ut.user_id = $1 AND (
		(t.media_type = 'movie' AND cardinality(ut.watched_at) > 0)
		OR ut.id IN (SELECT user_title_id FROM season_watches)
	)`

func (r *MoviesRepository) Stats(
	ctx context.Context,
	userID string,
) (*models.Stats, error) {
	//nolint:exhaustruct // filled below
	stats := &models.Stats{}
	var err error
	if stats.Months, err = r.monthWatches(ctx, userID); err != nil {
		return nil, err
	}
	if stats.Genres, err = r.topGenres(ctx, userID); err != nil {
		return nil, err
	}
	if stats.Ratings, err = r.ratingCounts(ctx, userID); err != nil {
		return nil, err
	}
	err = r.db.QueryRow(ctx, `
		WITH season_watches AS (`+seasonWatches+`),
			watched AS (`+watchedTitles+`)
		SELECT
			(SELECT count(*) FROM watched WHERE media_type = 'movie'),
			(SELECT count(*) FROM movies.user_titles ut
				JOIN movies.titles t ON t.id = ut.title_id
				WHERE ut.user_id = $1 AND t.media_type = 'series'
					AND ut.status = 'watched'),
			(SELECT count(*) FROM season_watches)`,
		userID,
	).Scan(&stats.MoviesWatched, &stats.SeriesWatched, &stats.SeasonsWatched)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *MoviesRepository) monthWatches(
	ctx context.Context,
	userID string,
) ([]models.MonthWatches, error) {
	rows, err := r.db.Query(ctx, `
		WITH season_watches AS (`+seasonWatches+`),
		months AS (
			SELECT generate_series(
				date_trunc('month', now() AT TIME ZONE $2) - interval '11 months',
				date_trunc('month', now() AT TIME ZONE $2),
				interval '1 month'
			) AS m
		),
		movie_months AS (
			SELECT date_trunc('month', w AT TIME ZONE $2) AS m
			FROM movies.user_titles ut
			JOIN movies.titles t ON t.id = ut.title_id AND t.media_type = 'movie'
			CROSS JOIN LATERAL unnest(ut.watched_at) AS w
			WHERE ut.user_id = $1 AND w IS NOT NULL
		),
		season_months AS (
			SELECT date_trunc('month', w AT TIME ZONE $2) AS m
			FROM season_watches sw
			CROSS JOIN LATERAL unnest(sw.watched_at) AS w
			WHERE w IS NOT NULL
		)
		SELECT to_char(months.m, 'YYYY-MM'),
			(SELECT count(*) FROM movie_months mm WHERE mm.m = months.m),
			(SELECT count(*) FROM season_months sm WHERE sm.m = months.m)
		FROM months
		ORDER BY months.m`,
		userID, statsTimezone,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.MonthWatches
	for rows.Next() {
		var m models.MonthWatches
		if err = rows.Scan(&m.Month, &m.Movies, &m.Seasons); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MoviesRepository) topGenres(
	ctx context.Context,
	userID string,
) ([]models.GenreCount, error) {
	const topGenres = 10
	rows, err := r.db.Query(ctx, `
		WITH season_watches AS (`+seasonWatches+`),
			watched AS (`+watchedTitles+`)
		SELECT g, count(*)
		FROM watched CROSS JOIN LATERAL unnest(watched.genres) AS g
		GROUP BY g
		ORDER BY count(*) DESC, g
		LIMIT $2`,
		userID, topGenres,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.GenreCount
	for rows.Next() {
		var g models.GenreCount
		if err = rows.Scan(&g.Genre, &g.Count); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *MoviesRepository) ratingCounts(
	ctx context.Context,
	userID string,
) ([5]int, error) {
	var counts [5]int
	rows, err := r.db.Query(ctx, `
		SELECT rating, count(*)
		FROM movies.user_titles
		WHERE user_id = $1 AND rating IS NOT NULL
		GROUP BY rating`,
		userID,
	)
	if err != nil {
		return counts, err
	}
	defer rows.Close()

	for rows.Next() {
		var rating, n int
		if err = rows.Scan(&rating, &n); err != nil {
			return counts, err
		}
		counts[rating-1] = n
	}
	return counts, rows.Err()
}
