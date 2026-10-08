package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
// days ago, and ones someone is watching or has watched (series) or wants or
// is watching (providers), fetched over 20 hours ago (under the daily run,
// so each run catches them).
func (r *MoviesRepository) DueTitles(ctx context.Context) ([]models.TitleKey, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.media_type, t.tmdb_id
		FROM movies.titles t
		WHERE t.fetched_at < now() - interval '30 days'
			OR (t.fetched_at < now() - interval '20 hours'
				AND EXISTS (
					SELECT 1 FROM movies.user_titles ut
					WHERE ut.title_id = t.id
						AND (ut.status IN ('want', 'watching')
							OR (t.media_type = 'series' AND ut.status = 'watched'))
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

// ReplaceProviders makes a title's stored providers match providers.
func (r *MoviesRepository) ReplaceProviders(
	ctx context.Context,
	titleID uuid.UUID,
	providers []models.Provider,
) error {
	batch := &pgx.Batch{} //nolint:exhaustruct //QueuedQueries populated via Queue()
	batch.Queue(`DELETE FROM movies.title_providers WHERE title_id = $1`, titleID)
	for _, p := range providers {
		batch.Queue(`
			INSERT INTO movies.title_providers (title_id, offer_type, provider_id,
				provider_name, logo_path, display_priority)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT DO NOTHING`,
			titleID, p.OfferType, p.ID, p.Name, p.LogoPath, p.DisplayPriority,
		)
	}
	return r.db.SendBatch(ctx, batch).Close()
}

// ListProviders returns a title's providers by offer type (streaming first),
// then TMDB's display priority.
func (r *MoviesRepository) ListProviders(
	ctx context.Context,
	titleID uuid.UUID,
) ([]models.Provider, error) {
	rows, err := r.db.Query(ctx, `
		SELECT provider_id, provider_name, logo_path, offer_type, display_priority
		FROM movies.title_providers
		WHERE title_id = $1
		ORDER BY array_position(
				ARRAY['flatrate', 'free', 'ads', 'rent', 'buy'], offer_type),
			display_priority, provider_name`,
		titleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Provider
	for rows.Next() {
		var p models.Provider
		if err = rows.Scan(
			&p.ID, &p.Name, &p.LogoPath, &p.OfferType, &p.DisplayPriority,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
