package repositories

import (
	"context"

	"github.com/google/uuid"
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
		    (title, authors, cover_url, description, page_count,
		     category, source_url)
		VALUES ($1, COALESCE($2, '{}'::text[]), $3, $4, $5,
		        COALESCE(NULLIF($6, ''), 'book'), $7)
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
		book.Category,
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

// SetBookCategory re-categorizes a catalog row. category must already be
// validated against models.IsValidCategory.
func (repo *BooksRepository) SetBookCategory(
	ctx context.Context,
	bookID uuid.UUID,
	category string,
) error {
	query := `UPDATE reading.books SET category = $2 WHERE id = $1`
	_, err := repo.db.Exec(ctx, query, bookID, category)
	return postgres.PgxErrorToHTTPError(err)
}
