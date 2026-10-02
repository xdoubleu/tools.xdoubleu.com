package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

const readingStateColumns = `user_id, book_id, source, percent, location,
	kobo_location, read_at, updated_at`

type BookReadingStateRepository struct {
	db postgres.DB
}

func (r *BookReadingStateRepository) Upsert(
	ctx context.Context,
	state models.BookReadingState,
) error {
	_, err := r.UpsertReturningUpdatedAt(ctx, state)
	return err
}

// UpsertReturningUpdatedAt is Upsert returning the stored row's updated_at.
func (r *BookReadingStateRepository) UpsertReturningUpdatedAt(
	ctx context.Context,
	state models.BookReadingState,
) (time.Time, error) {
	query := `
		INSERT INTO books.book_reading_state
		    (user_id, book_id, source, percent, location, kobo_location, read_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, book_id) DO UPDATE
		    SET source = EXCLUDED.source,
		        percent = EXCLUDED.percent,
		        location = EXCLUDED.location,
		        kobo_location = EXCLUDED.kobo_location,
		        read_at = EXCLUDED.read_at,
		        updated_at = now()
		RETURNING updated_at
	`

	var updatedAt time.Time
	err := r.db.QueryRow(ctx, query,
		state.UserID,
		state.BookID,
		state.Source,
		state.Percent,
		state.Location,
		state.KoboLocation,
		state.ReadAt,
	).Scan(&updatedAt)
	return updatedAt, postgres.PgxErrorToHTTPError(err)
}

func (r *BookReadingStateRepository) Get(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.BookReadingState, error) {
	query := `
		SELECT ` + readingStateColumns + `
		FROM books.book_reading_state
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

// ListByUser returns all reading states for the given user, keyed by book ID.
func (r *BookReadingStateRepository) ListByUser(
	ctx context.Context,
	userID string,
) ([]models.BookReadingState, error) {
	query := `
		SELECT ` + readingStateColumns + `
		FROM books.book_reading_state
		WHERE user_id = $1
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var states []models.BookReadingState
	for rows.Next() {
		s, scanErr := scanReadingState(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		states = append(states, *s)
	}
	return states, rows.Err()
}

// DeleteByBook removes the reading state for a single book owned by userID.
func (r *BookReadingStateRepository) DeleteByBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `
		DELETE FROM books.book_reading_state
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
		&s.KoboLocation,
		&s.ReadAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &s, nil
}
