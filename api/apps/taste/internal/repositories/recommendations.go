package repositories

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/taste/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type RecommendationsRepository struct {
	db postgres.DB
}

func NewRecommendationsRepository(db postgres.DB) *RecommendationsRepository {
	return &RecommendationsRepository{db: db}
}

// List returns userID's picks, best first; an empty domain or track matches
// all.
func (r *RecommendationsRepository) List(
	ctx context.Context,
	userID, domain, track string,
) ([]models.Recommendation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, domain, track, title, creator, year, catalog_ref, url,
		       image_url, why, sources, score, generated_at
		FROM taste.recommendations
		WHERE user_id = $1 AND ($2 = '' OR domain = $2) AND ($3 = '' OR track = $3)
		ORDER BY score DESC, title`, userID, domain, track)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	var out []models.Recommendation
	for rows.Next() {
		var rec models.Recommendation
		if err = rows.Scan(
			&rec.ID, &rec.Domain, &rec.Track, &rec.Title, &rec.Creator, &rec.Year,
			&rec.CatalogRef, &rec.URL, &rec.ImageURL, &rec.Why, &rec.Sources,
			&rec.Score, &rec.GeneratedAt,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, rec)
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}

// Replace swaps userID's picks for recs and records the run as successful.
func (r *RecommendationsRepository) Replace(
	ctx context.Context,
	userID string,
	recs []models.Recommendation,
	at time.Time,
) error {
	batch := &pgx.Batch{} //nolint:exhaustruct // QueuedQueries populated via Queue()
	batch.Queue("DELETE FROM taste.recommendations WHERE user_id = $1", userID)
	for _, rec := range recs {
		batch.Queue(`
			INSERT INTO taste.recommendations
			    (user_id, domain, track, title, creator, year, catalog_ref, url,
			     image_url, why, sources, score, generated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			userID, rec.Domain, rec.Track, rec.Title, rec.Creator, rec.Year,
			rec.CatalogRef, rec.URL, rec.ImageURL, rec.Why, rec.Sources,
			rec.Score, at,
		)
	}
	batch.Queue(`
		UPDATE taste.profiles SET recommended_at = $2, recommendations_error = ''
		WHERE user_id = $1`, userID, at)
	// A batch runs in one implicit transaction.
	return postgres.PgxErrorToHTTPError(r.db.SendBatch(ctx, batch).Close())
}

// SaveError notes a failed run, keeping the previous picks.
func (r *RecommendationsRepository) SaveError(
	ctx context.Context,
	userID, note string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE taste.profiles SET recommendations_error = $2
		WHERE user_id = $1`, userID, note)
	return postgres.PgxErrorToHTTPError(err)
}

// Status returns when userID's picks were last generated (nil before the
// first run) and the last run's error note.
func (r *RecommendationsRepository) Status(
	ctx context.Context,
	userID string,
) (*time.Time, string, error) {
	var at *time.Time
	var note string
	err := r.db.QueryRow(ctx, `
		SELECT recommended_at, recommendations_error
		FROM taste.profiles WHERE user_id = $1`, userID,
	).Scan(&at, &note)
	if err == pgx.ErrNoRows { //nolint:errorlint // pgx returns it unwrapped
		return nil, "", nil
	}
	return at, note, postgres.PgxErrorToHTTPError(err)
}
