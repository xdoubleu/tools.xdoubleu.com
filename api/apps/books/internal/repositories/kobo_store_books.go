package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

// UpsertKoboStoreBook records a store book; a nil Owned keeps the stored one.
func (repo *BooksRepository) UpsertKoboStoreBook(
	ctx context.Context,
	userID string,
	sb models.KoboStoreBook,
) error {
	authors := sb.Authors
	if authors == nil {
		authors = []string{}
	}
	_, err := repo.db.Exec(ctx, `
		INSERT INTO books.kobo_store_books
		    (user_id, entitlement_id, isbn13, title, authors, owned)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6, false))
		ON CONFLICT (user_id, entitlement_id) DO UPDATE
		    SET isbn13 = EXCLUDED.isbn13,
		        title = EXCLUDED.title,
		        authors = EXCLUDED.authors,
		        owned = COALESCE($6, books.kobo_store_books.owned),
		        updated_at = now()
	`, userID, sb.EntitlementID, sb.ISBN13, sb.Title, authors, sb.Owned)
	return postgres.PgxErrorToHTTPError(err)
}

// GetKoboStoreBook returns a recorded store book, or ErrResourceNotFound.
func (repo *BooksRepository) GetKoboStoreBook(
	ctx context.Context,
	userID string,
	entitlementID string,
) (*models.KoboStoreBook, error) {
	var sb models.KoboStoreBook
	var owned bool
	err := repo.db.QueryRow(ctx, `
		SELECT entitlement_id, isbn13, title, authors, owned
		FROM books.kobo_store_books
		WHERE user_id = $1 AND entitlement_id = $2
	`, userID, entitlementID).Scan(
		&sb.EntitlementID, &sb.ISBN13, &sb.Title, &sb.Authors, &owned,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, database.ErrResourceNotFound
	}
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	sb.Owned = &owned
	return &sb, nil
}
