package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// PromoteToReading moves a user_book to "currently-reading" when its current
// status is "to-read" or "dropped". Any other status (already reading, read) is
// left unchanged. The update is a single atomic conditional statement so there is
// no read-modify-write race and no other fields are clobbered.
func (repo *BooksRepository) PromoteToReading(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `
		UPDATE books.user_books
		SET status = $3, updated_at = now()
		WHERE user_id = $1 AND book_id = $2
		  AND status IN ($4, $5)
	`
	_, err := repo.db.Exec(
		ctx, query,
		userID, bookID,
		models.StatusReading,
		models.StatusToRead,
		models.StatusDropped,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// UpdateProgress sets the reading-progress fields for a user_book. The caller is
// responsible for validating the mode and clamping the values.
func (repo *BooksRepository) UpdateProgress(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	mode string,
	currentPage int,
	progressPercent int,
) error {
	query := `
		UPDATE books.user_books
		SET progress_mode = $3,
		    current_page = $4,
		    progress_percent = $5,
		    updated_at = now()
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := repo.db.Exec(
		ctx, query, userID, bookID, mode, currentPage, progressPercent,
	)
	return postgres.PgxErrorToHTTPError(err)
}
