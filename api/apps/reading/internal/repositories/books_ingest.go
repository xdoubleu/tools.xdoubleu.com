package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/reading/internal/models"
)

// UpsertBookBySourceURL inserts a catalog row keyed on the canonical source
// URL, refreshing metadata on conflict — the source_url mirror of
// UpsertBook's isbn13 path. book.SourceURL must be non-nil.
func (repo *BooksRepository) UpsertBookBySourceURL(
	ctx context.Context,
	book models.Book,
) (*models.Book, error) {
	query := `
		INSERT INTO reading.books
		    (title, authors, cover_url, description, page_count, source_url)
		VALUES ($1, COALESCE($2, '{}'::text[]), $3, $4, $5, $6)
		ON CONFLICT (source_url) WHERE source_url IS NOT NULL
		DO UPDATE SET
		    title       = EXCLUDED.title,
		    authors     = EXCLUDED.authors,
		    cover_url   = COALESCE(EXCLUDED.cover_url, reading.books.cover_url),
		    description = COALESCE(
		        EXCLUDED.description, reading.books.description
		    ),
		    page_count  = COALESCE(EXCLUDED.page_count, reading.books.page_count),
		    updated_at  = now()
		RETURNING ` + bookColumns

	row := repo.db.QueryRow(ctx, query,
		book.Title,
		book.Authors,
		book.CoverURL,
		book.Description,
		book.PageCount,
		book.SourceURL,
	)

	book2, err := scanBook(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return book2, nil
}

// GetBookBySourceURL returns the catalog book with the given canonical source
// URL. Returns database.ErrResourceNotFound when no book matches.
func (repo *BooksRepository) GetBookBySourceURL(
	ctx context.Context,
	sourceURL string,
) (*models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM reading.books
		WHERE source_url = $1
	`

	row := repo.db.QueryRow(ctx, query, sourceURL)
	book, err := scanBook(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return book, nil
}

// SetBookContentHTML stores the readability-extracted article body for an
// in-app read, independent of any EPUB file that may or may not exist.
func (repo *BooksRepository) SetBookContentHTML(
	ctx context.Context,
	bookID uuid.UUID,
	html string,
) error {
	query := `UPDATE reading.books SET content_html = $2 WHERE id = $1`
	_, err := repo.db.Exec(ctx, query, bookID, html)
	return postgres.PgxErrorToHTTPError(err)
}

// GetBookContentHTML returns the stored article HTML for a book in the
// caller's own library. Returns database.ErrResourceNotFound if the book
// isn't in the user's library; the returned pointer is nil if no content was
// ever stored for it.
func (repo *BooksRepository) GetBookContentHTML(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*string, error) {
	query := `
		SELECT b.content_html
		FROM reading.user_books ub
		JOIN reading.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1 AND ub.book_id = $2
	`

	rows, err := repo.db.Query(ctx, query, userID, bookID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, database.ErrResourceNotFound
	}

	var html *string
	if err = rows.Scan(&html); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return html, nil
}
