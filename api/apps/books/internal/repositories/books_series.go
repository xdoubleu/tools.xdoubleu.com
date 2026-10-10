package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

// SetBookSeries overwrites a book's series; nil clears it.
func (repo *BooksRepository) SetBookSeries(
	ctx context.Context,
	bookID uuid.UUID,
	series *models.BookSeries,
) error {
	query := `
		UPDATE books.books
		SET series_name     = $2,
		    series_position = $3,
		    series_total    = $4,
		    updated_at      = now()
		WHERE id = $1
	`
	name, position, total := seriesColumns(series)
	_, err := repo.db.Exec(ctx, query, bookID, name, position, total)
	return postgres.PgxErrorToHTTPError(err)
}

// SetSeriesTotal records a series' main-volume count on every book in it.
func (repo *BooksRepository) SetSeriesTotal(
	ctx context.Context,
	name string,
	total int,
) error {
	query := `
		UPDATE books.books
		SET series_total = $2
		WHERE series_name = $1
		  AND series_total IS DISTINCT FROM $2
	`
	_, err := repo.db.Exec(ctx, query, name, total)
	return postgres.PgxErrorToHTTPError(err)
}

// ListUserBooksInSeries returns the user's books in the named series, ordered
// by position (unpositioned last), then title.
func (repo *BooksRepository) ListUserBooksInSeries(
	ctx context.Context,
	userID string,
	name string,
) ([]models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1 AND b.series_name = $2
		ORDER BY b.series_position NULLS LAST, b.title
	`
	return repo.queryUserBooks(ctx, query, userID, name)
}
