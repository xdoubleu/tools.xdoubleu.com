package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/movies/internal/models"
	"tools.xdoubleu.com/internal/database"
)

// UpsertSeasons makes a series' catalog seasons match seasons, deleting the
// ones no longer listed; the watch columns of seasons are ignored. Users'
// watches of a deleted season are kept, hidden until it returns.
func (r *MoviesRepository) UpsertSeasons(
	ctx context.Context,
	titleID uuid.UUID,
	seasons []models.Season,
) error {
	numbers := make([]int, len(seasons))
	for i, s := range seasons {
		numbers[i] = s.Number
	}
	batch := &pgx.Batch{} //nolint:exhaustruct //QueuedQueries populated via Queue()
	batch.Queue(`
		DELETE FROM movies.seasons
		WHERE title_id = $1 AND season_number <> ALL($2)`,
		titleID, numbers,
	)
	for _, s := range seasons {
		batch.Queue(`
			INSERT INTO movies.seasons
				(title_id, season_number, name, air_date, episode_count)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (title_id, season_number) DO UPDATE SET
				name = EXCLUDED.name,
				air_date = EXCLUDED.air_date,
				episode_count = EXCLUDED.episode_count`,
			titleID, s.Number, s.Name, s.AirDate, s.EpisodeCount,
		)
	}
	return r.db.SendBatch(ctx, batch).Close()
}

// ListSeasons returns the entry's catalog seasons with the user's watches, in
// season order. Another user's entry has none.
func (r *MoviesRepository) ListSeasons(
	ctx context.Context,
	userID string,
	entryID uuid.UUID,
) ([]models.Season, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.season_number, s.name, s.air_date, s.episode_count,
			COALESCE(us.watched_at, '{}')
		FROM movies.user_titles ut
		JOIN movies.seasons s ON s.title_id = ut.title_id
		LEFT JOIN movies.user_seasons us
			ON us.user_title_id = ut.id AND us.season_number = s.season_number
		WHERE ut.id = $1 AND ut.user_id = $2
		ORDER BY s.season_number`,
		entryID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []models.Season
	for rows.Next() {
		var s models.Season
		if err = rows.Scan(
			&s.Number, &s.Name, &s.AirDate, &s.EpisodeCount, &s.WatchedAt,
		); err != nil {
			return nil, err
		}
		seasons = append(seasons, s)
	}
	return seasons, rows.Err()
}

// SetSeasonWatchedAt replaces the user's watches of one season of their
// entry; an empty list unticks it.
func (r *MoviesRepository) SetSeasonWatchedAt(
	ctx context.Context,
	userID string,
	entryID uuid.UUID,
	number int,
	watchedAt []*time.Time,
) error {
	if len(watchedAt) == 0 {
		_, err := r.db.Exec(ctx, `
			DELETE FROM movies.user_seasons us
			USING movies.user_titles ut
			WHERE us.user_title_id = ut.id AND ut.id = $1 AND ut.user_id = $2
				AND us.season_number = $3`,
			entryID, userID, number,
		)
		return err
	}

	tag, err := r.db.Exec(ctx, `
		INSERT INTO movies.user_seasons (user_title_id, season_number, watched_at)
		SELECT ut.id, $3, $4 FROM movies.user_titles ut
		WHERE ut.id = $1 AND ut.user_id = $2
		ON CONFLICT (user_title_id, season_number) DO UPDATE SET
			watched_at = EXCLUDED.watched_at`,
		entryID, userID, number, watchedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// SetWatchedAt replaces the watches recorded on the entry itself (movies).
func (r *MoviesRepository) SetWatchedAt(
	ctx context.Context,
	userID string,
	entryID uuid.UUID,
	watchedAt []*time.Time,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE movies.user_titles SET watched_at = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2`,
		entryID, userID, watchedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}
