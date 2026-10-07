package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/pagination"
)

type MoviesRepository struct {
	db postgres.DB
}

// entryColumns excludes t.overview, which only GetEntry reads.
const entryColumns = `ut.id, ut.user_id, ut.status, ut.rating, ut.watched_at,
	ut.added_at, ut.updated_at, t.id, t.media_type, t.tmdb_id, t.title,
	t.original_title, t.release_date, t.poster_path, t.genres, t.runtime,
	t.season_count`

//nolint:gochecknoglobals // fixed lookup of whitelisted ORDER BY clauses
var orderBy = map[string]string{
	models.SortAdded:   "ut.added_at DESC, ut.id",
	models.SortTitle:   "lower(t.title), ut.id",
	models.SortRelease: "t.release_date DESC NULLS LAST, ut.id",
	models.SortRating:  "ut.rating DESC NULLS LAST, ut.added_at DESC, ut.id",
}

func scanEntry(row pgx.Row, extra ...any) (*models.Entry, error) {
	var e models.Entry
	dest := []any{
		&e.ID, &e.UserID, &e.Status, &e.Rating, &e.WatchedAt, &e.AddedAt, &e.UpdatedAt,
		&e.Title.ID, &e.Title.MediaType, &e.Title.TMDBID, &e.Title.Title,
		&e.Title.OriginalTitle, &e.Title.ReleaseDate, &e.Title.PosterPath,
		&e.Title.Genres, &e.Title.Runtime, &e.Title.SeasonCount,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, err
	}
	return &e, nil
}

// UpsertTitle inserts or refreshes a catalog row and returns its ID.
func (r *MoviesRepository) UpsertTitle(
	ctx context.Context,
	t models.Title,
) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		INSERT INTO movies.titles (media_type, tmdb_id, title, original_title,
			release_date, poster_path, overview, genres, runtime, season_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, '{}'::text[]), $9, $10)
		ON CONFLICT (media_type, tmdb_id) DO UPDATE SET
			title = EXCLUDED.title,
			original_title = EXCLUDED.original_title,
			release_date = EXCLUDED.release_date,
			poster_path = EXCLUDED.poster_path,
			overview = EXCLUDED.overview,
			genres = EXCLUDED.genres,
			runtime = EXCLUDED.runtime,
			season_count = EXCLUDED.season_count,
			fetched_at = now()
		RETURNING id`,
		t.MediaType, t.TMDBID, t.Title, t.OriginalTitle, t.ReleaseDate,
		t.PosterPath, t.Overview, t.Genres, t.Runtime, t.SeasonCount,
	).Scan(&id)
	return id, err
}

// AddEntry puts a title in the user's backlog, or updates its status when it
// is already there. unknownDate dates a new movie watch as unknown, not now.
func (r *MoviesRepository) AddEntry(
	ctx context.Context,
	userID string,
	titleID uuid.UUID,
	status string,
	unknownDate bool,
) (uuid.UUID, error) {
	var id uuid.UUID
	// A movie added as watched gets one watch; re-adding keeps existing
	// watches.
	err := r.db.QueryRow(ctx, `
		INSERT INTO movies.user_titles AS ut
			(user_id, title_id, status, watched_at)
		SELECT $1, t.id, $3, CASE
			WHEN $3 = 'watched' AND t.media_type = 'movie'
			THEN ARRAY[CASE WHEN $4 THEN NULL ELSE now() END]::timestamptz[]
			ELSE '{}'::timestamptz[]
		END
		FROM movies.titles t WHERE t.id = $2
		ON CONFLICT (user_id, title_id) DO UPDATE SET
			status = EXCLUDED.status,
			watched_at = CASE
				WHEN cardinality(ut.watched_at) = 0 THEN EXCLUDED.watched_at
				ELSE ut.watched_at
			END,
			updated_at = now()
		RETURNING id`,
		userID, titleID, status, unknownDate,
	).Scan(&id)
	return id, postgres.PgxErrorToHTTPError(err)
}

func (r *MoviesRepository) SetStatus(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	status string,
	unknownDate bool,
) error {
	// A movie becoming watched with no recorded watch gets one, dated now
	// unless unknownDate.
	tag, err := r.db.Exec(ctx, `
		UPDATE movies.user_titles ut
		SET status = $3,
			watched_at = CASE
				WHEN $3 = 'watched' AND t.media_type = 'movie'
					AND cardinality(ut.watched_at) = 0
				THEN ARRAY[CASE WHEN $4 THEN NULL ELSE now() END]::timestamptz[]
				ELSE ut.watched_at
			END,
			updated_at = now()
		FROM movies.titles t
		WHERE ut.id = $1 AND ut.user_id = $2 AND t.id = ut.title_id`,
		id, userID, status, unknownDate,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// SetRating stores the entry's rating; nil clears it.
func (r *MoviesRepository) SetRating(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	rating *int,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE movies.user_titles SET rating = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2`,
		id, userID, rating,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

func (r *MoviesRepository) DeleteEntry(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM movies.user_titles WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// GetEntry returns the entry with the title's overview filled in. Another
// user's entry reads as not found.
func (r *MoviesRepository) GetEntry(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) (*models.Entry, error) {
	var overview string
	e, err := scanEntry(r.db.QueryRow(ctx, `
		SELECT `+entryColumns+`, t.overview
		FROM movies.user_titles ut
		JOIN movies.titles t ON t.id = ut.title_id
		WHERE ut.id = $1 AND ut.user_id = $2`,
		id, userID,
	), &overview)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	e.Title.Overview = overview
	return e, nil
}

func (r *MoviesRepository) ListEntries(
	ctx context.Context,
	userID string,
	f models.ListFilter,
) ([]models.Entry, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(f.Limit)
	order, ok := orderBy[f.Sort]
	if !ok {
		order = orderBy[models.SortAdded]
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+entryColumns+`
		FROM movies.user_titles ut
		JOIN movies.titles t ON t.id = ut.title_id
		WHERE ut.user_id = $1
			AND ($2 = '' OR ut.status = $2)
			AND ($3 = '' OR t.media_type = $3)
		ORDER BY `+order+`
		LIMIT $4 OFFSET $5`,
		userID, f.Status, f.MediaType, sqlLimit, f.Offset,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var result []models.Entry
	for rows.Next() {
		e, scanErr := scanEntry(rows)
		if scanErr != nil {
			return nil, false, scanErr
		}
		result = append(result, *e)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}

	page, hasMore := pagination.Split(result, safeLimit)
	return page, hasMore, nil
}

// StatusesByKey returns the user's status for each given title already in
// their backlog.
func (r *MoviesRepository) StatusesByKey(
	ctx context.Context,
	userID string,
	keys []models.TitleKey,
) (map[models.TitleKey]string, error) {
	types := make([]string, len(keys))
	ids := make([]int64, len(keys))
	for i, k := range keys {
		types[i], ids[i] = k.MediaType, k.TMDBID
	}

	rows, err := r.db.Query(ctx, `
		SELECT t.media_type, t.tmdb_id, ut.status
		FROM unnest($2::text[], $3::bigint[]) AS k(media_type, tmdb_id)
		JOIN movies.titles t
			ON t.media_type = k.media_type AND t.tmdb_id = k.tmdb_id
		JOIN movies.user_titles ut
			ON ut.title_id = t.id AND ut.user_id = $1`,
		userID, types, ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[models.TitleKey]string)
	for rows.Next() {
		var k models.TitleKey
		var status string
		if err = rows.Scan(&k.MediaType, &k.TMDBID, &status); err != nil {
			return nil, err
		}
		out[k] = status
	}
	return out, rows.Err()
}
