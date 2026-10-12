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

const (
	recommendationColumns = `id, domain, track, title, creator, year,
	catalog_ref, url, image_url, why, sources, score, generated_at, feedback,
	saved, accepted, on_my_services`
	// touched matches picks the user acted on; a new run keeps them.
	touched = `(feedback <> '' OR saved OR accepted)`
)

// List returns userID's picks from the latest run, or every saved pick when
// savedOnly, best first; an empty domain or track matches all.
func (r *RecommendationsRepository) List(
	ctx context.Context,
	userID, domain, track string,
	savedOnly bool,
) ([]models.Recommendation, error) {
	return r.query(ctx, `
		SELECT `+recommendationColumns+`
		FROM taste.recommendations
		WHERE user_id = $1 AND ($2 = '' OR domain = $2) AND ($3 = '' OR track = $3)
		  AND CASE WHEN $4 THEN saved ELSE generated_at = (
		      SELECT recommended_at FROM taste.profiles WHERE user_id = $1
		  ) END
		ORDER BY score DESC, title`, userID, domain, track, savedOnly)
}

// Touched returns what generation reads of every pick userID acted on,
// newest feedback first.
func (r *RecommendationsRepository) Touched(
	ctx context.Context,
	userID string,
) ([]models.Recommendation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, domain, track, title, catalog_ref, feedback
		FROM taste.recommendations
		WHERE user_id = $1 AND `+touched+`
		ORDER BY feedback_at DESC NULLS LAST, generated_at DESC`, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	var out []models.Recommendation
	for rows.Next() {
		var rec models.Recommendation
		if err = rows.Scan(
			&rec.ID, &rec.Domain, &rec.Track, &rec.Title, &rec.CatalogRef, &rec.Feedback,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, rec)
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}

func (r *RecommendationsRepository) query(
	ctx context.Context,
	sql string,
	args ...any,
) ([]models.Recommendation, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	var out []models.Recommendation
	for rows.Next() {
		rec, scanErr := scanRecommendation(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, *rec)
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}

func scanRecommendation(row pgx.Row) (*models.Recommendation, error) {
	var rec models.Recommendation
	err := row.Scan(
		&rec.ID, &rec.Domain, &rec.Track, &rec.Title, &rec.Creator, &rec.Year,
		&rec.CatalogRef, &rec.URL, &rec.ImageURL, &rec.Why, &rec.Sources,
		&rec.Score, &rec.GeneratedAt, &rec.Feedback, &rec.Saved, &rec.Accepted,
		&rec.OnMyServices,
	)
	return &rec, err
}

// Get returns userID's pick id.
func (r *RecommendationsRepository) Get(
	ctx context.Context,
	userID, id string,
) (*models.Recommendation, error) {
	rec, err := scanRecommendation(r.db.QueryRow(ctx, `
		SELECT `+recommendationColumns+`
		FROM taste.recommendations WHERE user_id = $1 AND id = $2`, userID, id))
	return rec, postgres.PgxErrorToHTTPError(err)
}

// SetFeedback records feedback on userID's pick id and returns it.
func (r *RecommendationsRepository) SetFeedback(
	ctx context.Context,
	userID, id, feedback string,
	at time.Time,
) (*models.Recommendation, error) {
	return r.update(ctx, `feedback = $3, feedback_at = $4`, userID, id, feedback, at)
}

// SetSaved saves or unsaves userID's pick id and returns it.
func (r *RecommendationsRepository) SetSaved(
	ctx context.Context,
	userID, id string,
	saved bool,
) (*models.Recommendation, error) {
	return r.update(ctx, `saved = $3`, userID, id, saved)
}

// SetAccepted marks userID's pick id as added to its backlog and returns it.
func (r *RecommendationsRepository) SetAccepted(
	ctx context.Context,
	userID, id string,
) (*models.Recommendation, error) {
	return r.update(ctx, `accepted = TRUE`, userID, id)
}

func (r *RecommendationsRepository) update(
	ctx context.Context,
	set, userID, id string,
	args ...any,
) (*models.Recommendation, error) {
	rec, err := scanRecommendation(r.db.QueryRow(ctx, `
		UPDATE taste.recommendations SET `+set+`
		WHERE user_id = $1 AND id = $2
		RETURNING `+recommendationColumns, append([]any{userID, id}, args...)...))
	return rec, postgres.PgxErrorToHTTPError(err)
}

// HitRates counts userID's hits and misses per domain and track.
func (r *RecommendationsRepository) HitRates(
	ctx context.Context,
	userID string,
) ([]models.HitRate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT domain, track,
		       COUNT(*) FILTER (WHERE feedback = 'hit'),
		       COUNT(*) FILTER (WHERE feedback = 'miss')
		FROM taste.recommendations
		WHERE user_id = $1 AND feedback IN ('hit', 'miss')
		GROUP BY domain, track`, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()
	var out []models.HitRate
	for rows.Next() {
		var h models.HitRate
		if err = rows.Scan(&h.Domain, &h.Track, &h.Hits, &h.Misses); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, h)
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}

// Replace swaps userID's untouched picks for recs and records the run as
// successful; picks the user acted on stay.
func (r *RecommendationsRepository) Replace(
	ctx context.Context,
	userID string,
	recs []models.Recommendation,
	at time.Time,
) error {
	batch := &pgx.Batch{} //nolint:exhaustruct // QueuedQueries populated via Queue()
	batch.Queue(`
		DELETE FROM taste.recommendations
		WHERE user_id = $1 AND NOT `+touched, userID)
	for _, rec := range recs {
		batch.Queue(`
			INSERT INTO taste.recommendations
			    (user_id, domain, track, title, creator, year, catalog_ref, url,
			     image_url, why, sources, score, generated_at, on_my_services)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			userID, rec.Domain, rec.Track, rec.Title, rec.Creator, rec.Year,
			rec.CatalogRef, rec.URL, rec.ImageURL, rec.Why, rec.Sources,
			rec.Score, at, rec.OnMyServices,
		)
	}
	// The latest run is the one whose time matches the profile's, so the
	// profile row must exist.
	batch.Queue(`
		INSERT INTO taste.profiles (user_id, recommended_at) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET recommended_at = $2, recommendations_error = ''`, userID, at)
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
