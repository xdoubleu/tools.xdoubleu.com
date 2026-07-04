package repositories

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/books/internal/models"
)

// bookColumns is the standalone column list for books.books selects. The order
// must match scanBook.
const bookColumns = `id, title, authors, isbn13, cover_url, description,
	page_count, created_at, updated_at,
	openlibrary_found, googlebooks_found, unicat_found, last_resync_at`

// userBookColumns is the joined column list for user_book selects. The order must
// match scanUserBookWithBook (user_book columns first, then the joined book).
const userBookColumns = `ub.id, ub.user_id, ub.book_id, ub.status, ub.tags,
	ub.shelf_positions, ub.rating, ub.finished_at, ub.progress_mode,
	ub.current_page, ub.progress_percent, ub.added_at, ub.updated_at,
	b.id, b.title, b.authors, b.isbn13, b.cover_url, b.description,
	b.page_count, b.created_at, b.updated_at`

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func scanBook(row pgx.Row) (*models.Book, error) {
	var book models.Book

	err := row.Scan(
		&book.ID,
		&book.Title,
		&book.Authors,
		&book.ISBN13,
		&book.CoverURL,
		&book.Description,
		&book.PageCount,
		&book.CreatedAt,
		&book.UpdatedAt,
		&book.OpenLibraryFound,
		&book.GoogleBooksFound,
		&book.UniCatFound,
		&book.LastResyncAt,
	)
	if err != nil {
		return nil, err
	}

	return &book, nil
}

func scanUserBookWithBook(rows pgx.Rows) (models.UserBook, error) {
	var ub models.UserBook
	var book models.Book
	var posJSON []byte

	err := rows.Scan(
		&ub.ID,
		&ub.UserID,
		&ub.BookID,
		&ub.Status,
		&ub.Tags,
		&posJSON,
		&ub.Rating,
		&ub.FinishedAt,
		&ub.ProgressMode,
		&ub.CurrentPage,
		&ub.ProgressPercent,
		&ub.AddedAt,
		&ub.UpdatedAt,
		&book.ID,
		&book.Title,
		&book.Authors,
		&book.ISBN13,
		&book.CoverURL,
		&book.Description,
		&book.PageCount,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		return models.UserBook{}, err
	}

	ub.ShelfPositions = map[string]int{}
	if len(posJSON) > 0 {
		if jsonErr := json.Unmarshal(posJSON, &ub.ShelfPositions); jsonErr != nil {
			return models.UserBook{}, jsonErr
		}
	}

	ub.Book = &book
	return ub, nil
}
