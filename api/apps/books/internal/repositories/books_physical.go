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

// GetPhysicalCopy returns the user's physical copy details, defaulting to the
// catalog's page count and page 1.
func (repo *BooksRepository) GetPhysicalCopy(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.PhysicalCopy, error) {
	query := `
		SELECT COALESCE(ub.physical_page_count, b.page_count, 0),
		       COALESCE(ub.physical_text_start_page, 1),
		       ub.physical_anchors,
		       $3 = ANY(ub.tags)
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1 AND ub.book_id = $2
	`
	var c models.PhysicalCopy
	err := repo.db.QueryRow(ctx, query, userID, bookID, models.TagOwnPhysical).Scan(
		&c.PageCount, &c.TextStartPage, &c.Anchors, &c.Syncable)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, database.ErrResourceNotFound
	}
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &c, nil
}

// UpdatePhysicalCopy stores the copy's details; a zero page count or text
// start page falls back to its default.
func (repo *BooksRepository) UpdatePhysicalCopy(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	c models.PhysicalCopy,
) error {
	anchors := c.Anchors
	if anchors == nil {
		anchors = []models.PhysicalAnchor{}
	}
	tag, err := repo.db.Exec(ctx, `
		UPDATE books.user_books
		SET physical_page_count = NULLIF($3, 0),
		    physical_text_start_page = NULLIF($4, 0),
		    physical_anchors = $5,
		    updated_at = now()
		WHERE user_id = $1 AND book_id = $2
	`, userID, bookID, c.PageCount, c.TextStartPage, anchors)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// SetCurrentPageIfLatest sets current_page while the reading state is still
// the digital write stored at stateUpdatedAt, so a newer write (a page entered
// by hand included) is never overwritten.
func (repo *BooksRepository) SetCurrentPageIfLatest(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	page int,
	stateUpdatedAt time.Time,
) error {
	_, err := repo.db.Exec(ctx, `
		UPDATE books.user_books ub
		SET current_page = $3, updated_at = now()
		WHERE ub.user_id = $1 AND ub.book_id = $2
		  AND EXISTS (
		      SELECT 1 FROM books.book_reading_state rs
		      WHERE rs.user_id = $1 AND rs.book_id = $2
		        AND rs.source <> $5 AND rs.updated_at = $4
		  )
	`, userID, bookID, page, stateUpdatedAt, models.ReadingSourcePhysical)
	return postgres.PgxErrorToHTTPError(err)
}
