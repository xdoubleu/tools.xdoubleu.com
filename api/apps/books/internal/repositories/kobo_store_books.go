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

// RecordKoboStoreOutcome stores a store book's last mirrored reading and its
// outcome.
func (repo *BooksRepository) RecordKoboStoreOutcome(
	ctx context.Context,
	userID string,
	reading models.KoboStoreReading,
	outcome models.KoboStoreOutcome,
) error {
	_, err := repo.db.Exec(ctx, `
		UPDATE books.kobo_store_books
		SET last_percent = $3, last_read_at = $4, last_outcome = $5,
		    last_mirrored_at = now()
		WHERE user_id = $1 AND entitlement_id = $2
	`, userID, reading.EntitlementID, reading.Percent, reading.ReadAt, string(outcome))
	return postgres.PgxErrorToHTTPError(err)
}

// ListKoboStoreBooks returns the user's recorded store books, most recently
// mirrored or updated first. Match is left nil.
func (repo *BooksRepository) ListKoboStoreBooks(
	ctx context.Context,
	userID string,
) ([]models.KoboStoreBookStatus, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT entitlement_id, isbn13, title, authors, owned,
		       last_percent, last_read_at, last_outcome, last_mirrored_at,
		       updated_at
		FROM books.kobo_store_books
		WHERE user_id = $1
		ORDER BY GREATEST(last_mirrored_at, updated_at) DESC, entitlement_id
	`, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.KoboStoreBookStatus
	for rows.Next() {
		var st models.KoboStoreBookStatus
		var owned bool
		var outcome string
		if err = rows.Scan(
			&st.EntitlementID, &st.ISBN13, &st.Title, &st.Authors, &owned,
			&st.LastPercent, &st.LastReadAt, &outcome, &st.LastMirroredAt,
			&st.UpdatedAt,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		st.Owned = &owned
		st.LastOutcome = models.KoboStoreOutcome(outcome)
		out = append(out, st)
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}
