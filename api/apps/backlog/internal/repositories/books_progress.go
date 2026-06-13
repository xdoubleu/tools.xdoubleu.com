package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
)

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
		UPDATE backlog.user_books
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
