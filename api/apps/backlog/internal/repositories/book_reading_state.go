package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

const readingStateColumns = `user_id, book_id, source, percent, location, updated_at`

type BookReadingStateRepository struct {
	db postgres.DB
}

func (r *BookReadingStateRepository) Upsert(
	ctx context.Context,
	state models.BookReadingState,
) error {
	query := `
		INSERT INTO backlog.book_reading_state
		    (user_id, book_id, source, percent, location)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, book_id) DO UPDATE
		    SET source = EXCLUDED.source,
		        percent = EXCLUDED.percent,
		        location = EXCLUDED.location,
		        updated_at = now()
	`

	_, err := r.db.Exec(ctx, query,
		state.UserID,
		state.BookID,
		state.Source,
		state.Percent,
		state.Location,
	)
	return postgres.PgxErrorToHTTPError(err)
}

func (r *BookReadingStateRepository) Get(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookReadingState, error) {
	query := `
		SELECT ` + readingStateColumns + `
		FROM backlog.book_reading_state
		WHERE user_id = $1 AND book_id = $2
	`

	row := r.db.QueryRow(ctx, query, userID, bookID)
	state, err := scanReadingState(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, database.ErrResourceNotFound
		}
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return state, nil
}

func (r *BookReadingStateRepository) DeleteByUser(
	ctx context.Context,
	userID string,
) error {
	query := `DELETE FROM backlog.book_reading_state WHERE user_id = $1`
	_, err := r.db.Exec(ctx, query, userID)
	return postgres.PgxErrorToHTTPError(err)
}

// DeleteByBook removes the reading state for a single book owned by userID.
func (r *BookReadingStateRepository) DeleteByBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `
		DELETE FROM backlog.book_reading_state
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := r.db.Exec(ctx, query, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}

func scanReadingState(row pgx.Row) (*models.BookReadingState, error) {
	var s models.BookReadingState

	err := row.Scan(
		&s.UserID,
		&s.BookID,
		&s.Source,
		&s.Percent,
		&s.Location,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &s, nil
}
