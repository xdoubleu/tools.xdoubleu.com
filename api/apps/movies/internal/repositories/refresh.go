package repositories

import (
	"context"

	"tools.xdoubleu.com/apps/movies/internal/models"
)

// DeleteOrphanTitles deletes catalog titles no backlog holds, which TMDB's
// terms don't let us cache, and returns how many it deleted. Titles fetched
// within the day are kept: one may be mid-add.
func (r *MoviesRepository) DeleteOrphanTitles(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM movies.titles t
		WHERE t.fetched_at < now() - interval '1 day'
			AND NOT EXISTS (
				SELECT 1 FROM movies.user_titles ut WHERE ut.title_id = t.id
			)`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DueTitles returns the titles to re-fetch from TMDB: any fetched over 30
// days ago, and series someone is watching or has watched fetched over 20
// hours ago (under the daily run, so each run catches them), so new seasons
// show up.
func (r *MoviesRepository) DueTitles(ctx context.Context) ([]models.TitleKey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.media_type, t.tmdb_id
		FROM movies.titles t
		WHERE t.fetched_at < now() - interval '30 days'
			OR (t.media_type = 'series'
				AND t.fetched_at < now() - interval '20 hours'
				AND EXISTS (
					SELECT 1 FROM movies.user_titles ut
					WHERE ut.title_id = t.id
						AND ut.status IN ('watching', 'watched')
				))
		ORDER BY t.fetched_at, t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.TitleKey
	for rows.Next() {
		var k models.TitleKey
		if err = rows.Scan(&k.MediaType, &k.TMDBID); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
