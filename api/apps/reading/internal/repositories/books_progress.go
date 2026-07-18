package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/reading/internal/models"
)

// UpdateLibraryProgress records reader-reported percent progress on a user_book:
// switches it to percent mode with the given progress_percent, and promotes status
// to "currently-reading" when it was still "to-read" or "dropped" (any other status,
// e.g. already reading or read, is left unchanged). current_page is untouched — Kobo
// only reports a percent. Single atomic statement, no read-modify-write race, no
// other fields clobbered.
func (repo *BooksRepository) UpdateLibraryProgress(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	percent int,
) error {
	query := `
		UPDATE reading.user_books
		SET progress_percent = $3,
		    progress_mode = $4,
		    status = CASE WHEN status IN ($6, $7) THEN $5 ELSE status END,
		    updated_at = now()
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := repo.db.Exec(
		ctx, query,
		userID, bookID, percent,
		models.ProgressModePercent,
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
		UPDATE reading.user_books
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
