package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/pagination"
)

type BooksRepository struct {
	db postgres.DB
}

func (repo *BooksRepository) UpsertBook(
	ctx context.Context,
	book models.Book,
) (*models.Book, error) {
	// Try match by ISBN13 first, then fall back to title+first author.
	query := `
		INSERT INTO books.books
		    (title, authors, isbn13, cover_url, description, page_count,
		     metadata_source, source_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL
		DO UPDATE SET
		    title         = EXCLUDED.title,
		    authors       = EXCLUDED.authors,
		    cover_url     = COALESCE(EXCLUDED.cover_url, books.books.cover_url),
		    description   = COALESCE(EXCLUDED.description, books.books.description),
		    page_count    = COALESCE(EXCLUDED.page_count, books.books.page_count),
		    metadata_source = COALESCE(
		        EXCLUDED.metadata_source, books.books.metadata_source
		    ),
		    updated_at    = now()
		RETURNING ` + bookColumns

	row := repo.db.QueryRow(ctx, query,
		book.Title,
		book.Authors,
		book.ISBN13,
		book.CoverURL,
		book.Description,
		book.PageCount,
		book.MetadataSource,
		book.SourceURL,
	)

	return scanBook(row)
}

func (repo *BooksRepository) FindBookByTitleAndAuthor(
	ctx context.Context,
	title string,
	author string,
) (*models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		WHERE title = $1 AND $2 = ANY(authors)
		LIMIT 1
	`

	row := repo.db.QueryRow(ctx, query, title, author)
	book, err := scanBook(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return book, nil
}

// GetBookByID returns the book with the given ID, or ErrResourceNotFound.
func (repo *BooksRepository) GetBookByID(
	ctx context.Context,
	bookID uuid.UUID,
) (*models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		WHERE id = $1
	`

	row := repo.db.QueryRow(ctx, query, bookID)
	book, err := scanBook(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return book, nil
}

// GetCatalogBookByISBN13 returns the catalog book with the ISBN-13, or
// ErrResourceNotFound.
func (repo *BooksRepository) GetCatalogBookByISBN13(
	ctx context.Context,
	isbn13 string,
) (*models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		WHERE isbn13 = $1
	`

	row := repo.db.QueryRow(ctx, query, isbn13)
	book, err := scanBook(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return book, nil
}

// UpdateBookByID overwrites a book's catalog fields by primary key only, never
// the isbn13 index, so it is safe when the resolved ISBN differs.
func (repo *BooksRepository) UpdateBookByID(
	ctx context.Context,
	book models.Book,
) error {
	query := `
		UPDATE books.books
		SET
		    title         = $2,
		    authors       = $3,
		    isbn13        = $4,
		    cover_url     = $5,
		    description   = $6,
		    page_count    = $7,
		    source_url    = COALESCE($8, source_url),
		    updated_at    = now()
		WHERE id = $1
	`

	_, err := repo.db.Exec(ctx, query,
		book.ID,
		book.Title,
		book.Authors,
		book.ISBN13,
		book.CoverURL,
		book.Description,
		book.PageCount,
		book.SourceURL,
	)

	return postgres.PgxErrorToHTTPError(err)
}

// DeleteOrphanedBook deletes a catalog book only when no user_books row
// references it, reporting whether it did (so callers clean up R2).
func (repo *BooksRepository) DeleteOrphanedBook(
	ctx context.Context,
	bookID uuid.UUID,
) (bool, error) {
	query := `
		DELETE FROM books.books
		WHERE id = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM books.user_books WHERE book_id = $1
		  )
	`

	tag, err := repo.db.Exec(ctx, query, bookID)
	if err != nil {
		return false, postgres.PgxErrorToHTTPError(err)
	}
	return tag.RowsAffected() > 0, nil
}

func (repo *BooksRepository) UpsertUserBook(
	ctx context.Context,
	ub models.UserBook,
) error {
	posJSON, err := json.Marshal(ub.ShelfPositions)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO books.user_books
		    (user_id, book_id, status, tags, shelf_positions,
		     rating, finished_at, added_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()))
		ON CONFLICT (user_id, book_id) DO UPDATE SET
		    status          = EXCLUDED.status,
		    tags            = EXCLUDED.tags,
		    shelf_positions = EXCLUDED.shelf_positions,
		    rating          = COALESCE(EXCLUDED.rating, books.user_books.rating),
		    finished_at     = EXCLUDED.finished_at,
		    updated_at      = now()
	`

	_, err = repo.db.Exec(ctx, query,
		ub.UserID,
		ub.BookID,
		ub.Status,
		ub.Tags,
		string(posJSON),
		ub.Rating,
		ub.FinishedAt,
		nullTime(ub.AddedAt),
	)

	return postgres.PgxErrorToHTTPError(err)
}

// UpdateUserBookAddedAt overwrites added_at on an existing user_book; the
// upsert's ON CONFLICT never touches added_at.
func (repo *BooksRepository) UpdateUserBookAddedAt(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	addedAt time.Time,
) error {
	query := `
		UPDATE books.user_books SET added_at = $3
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := repo.db.Exec(ctx, query, userID, bookID, addedAt)
	return postgres.PgxErrorToHTTPError(err)
}

func (repo *BooksRepository) GetByStatus(
	ctx context.Context,
	userID string,
	status string,
) ([]models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1 AND ub.status = $2
		ORDER BY b.title
	`

	return repo.queryUserBooks(ctx, query, userID, status)
}

func (repo *BooksRepository) GetLibrary(
	ctx context.Context,
	userID string,
) ([]models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1
		ORDER BY b.title
	`

	return repo.queryUserBooks(ctx, query, userID)
}

func (repo *BooksRepository) GetFinishedDates(
	ctx context.Context,
	userID string,
) ([]time.Time, error) {
	query := `
		SELECT UNNEST(ub.finished_at) AS finished_date
		FROM books.user_books ub
		WHERE ub.user_id = $1 AND ub.status = 'read'
		ORDER BY finished_date
	`

	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var t time.Time
		if err = rows.Scan(&t); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		dates = append(dates, t)
	}

	return dates, rows.Err()
}

func (repo *BooksRepository) queryUserBooks(
	ctx context.Context,
	query string,
	args ...any,
) ([]models.UserBook, error) {
	rows, err := repo.db.Query(ctx, query, args...)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var userBooks []models.UserBook
	for rows.Next() {
		ub, scanErr := scanUserBookWithBook(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		userBooks = append(userBooks, ub)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return userBooks, nil
}

//nolint:funlen // two-phase batch upsert; hard to split without obscuring the boundary
func (repo *BooksRepository) BatchUpsert(
	ctx context.Context,
	userID string,
	books []models.Book,
	userBooks []models.UserBook,
) error {
	if len(books) == 0 {
		return nil
	}

	if len(books) != len(userBooks) {
		return fmt.Errorf(
			"books and userBooks length mismatch: %d vs %d",
			len(books),
			len(userBooks),
		)
	}

	// 1. Upsert books.
	upsertBookQuery := `
		INSERT INTO books.books
		    (title, authors, isbn13, cover_url, description, page_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL
		DO UPDATE SET
		    title         = EXCLUDED.title,
		    authors       = EXCLUDED.authors,
		    cover_url     = COALESCE(EXCLUDED.cover_url, books.books.cover_url),
		    description   = COALESCE(EXCLUDED.description, books.books.description),
		    page_count    = COALESCE(EXCLUDED.page_count, books.books.page_count),
		    updated_at    = now()
		RETURNING id
	`

	bookIDs := make([]string, len(books))

	batch := &pgx.Batch{} //nolint:exhaustruct //QueuedQueries populated via Queue()

	for _, book := range books {
		batch.Queue(
			upsertBookQuery,
			book.Title,
			book.Authors,
			book.ISBN13,
			book.CoverURL,
			book.Description,
			book.PageCount,
		)
	}

	br := repo.db.SendBatch(ctx, batch)

	for i := 0; i < len(books); i++ {
		if err := br.QueryRow().Scan(&bookIDs[i]); err != nil {
			return fmt.Errorf("book upsert failed at index %d: %w", i, err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("book batch close: %w", err)
	}

	// 2. Assign book IDs.
	for i := range userBooks {
		userBooks[i].BookID = uuid.MustParse(bookIDs[i])
	}

	// 3. Upsert user_books.
	upsertUserBookQuery := `
		INSERT INTO books.user_books
		    (user_id, book_id, status, tags, shelf_positions,
		     rating, finished_at, added_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE($8, now()))
		ON CONFLICT (user_id, book_id) DO UPDATE SET
		    status          = EXCLUDED.status,
		    tags            = EXCLUDED.tags,
		    shelf_positions = EXCLUDED.shelf_positions,
		    rating          = COALESCE(EXCLUDED.rating, books.user_books.rating),
		    finished_at     = EXCLUDED.finished_at,
		    added_at        = COALESCE(books.user_books.added_at, EXCLUDED.added_at),
		    updated_at      = now()
	`

	batch = &pgx.Batch{} //nolint:exhaustruct //QueuedQueries populated via Queue()

	for _, ub := range userBooks {
		posJSON, marshalErr := json.Marshal(ub.ShelfPositions)
		if marshalErr != nil {
			return fmt.Errorf("marshal shelf positions: %w", marshalErr)
		}
		batch.Queue(
			upsertUserBookQuery,
			userID,
			ub.BookID,
			ub.Status,
			ub.Tags,
			string(posJSON),
			ub.Rating,
			ub.FinishedAt,
			nullTime(ub.AddedAt),
		)
	}

	br = repo.db.SendBatch(ctx, batch)

	for i := 0; i < len(userBooks); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("user_book upsert failed at index %d: %w", i, err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("user_book batch close: %w", err)
	}

	return nil
}

// GetUserBook fetches a single user_book by user and book ID.
func (repo *BooksRepository) GetUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
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

	ub, err := scanUserBookWithBook(rows)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &ub, nil
}

// SearchLibrary requires every query word to match the title or an author, so
// "Dune Herbert" matches title "Dune" by author "Herbert".
func (repo *BooksRepository) SearchLibrary(
	ctx context.Context,
	userID string,
	query string,
	limit int32,
	offset int32,
) ([]models.UserBook, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)
	tokens := strings.Fields(query)

	q := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1
		  AND (
		        SELECT bool_and(
		            b.title ILIKE '%' || t || '%'
		            OR EXISTS (
		                SELECT 1 FROM UNNEST(b.authors) a WHERE a ILIKE '%' || t || '%'
		            )
		        )
		        FROM UNNEST($2::text[]) AS t
		  )
		ORDER BY b.title, b.id
		LIMIT $3 OFFSET $4
	`

	rows, err := repo.queryUserBooks(ctx, q, userID, tokens, sqlLimit, offset)
	if err != nil {
		return nil, false, err
	}

	page, hasMore := pagination.Split(rows, safeLimit)
	return page, hasMore, nil
}

// FindUserBookByISBN13 finds the user's library entry for an ISBN13.
func (repo *BooksRepository) FindUserBookByISBN13(
	ctx context.Context,
	userID string,
	isbn13 string,
) (*models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1 AND b.isbn13 = $2
		LIMIT 1
	`

	rows, err := repo.db.Query(ctx, query, userID, isbn13)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, database.ErrResourceNotFound
	}

	ub, err := scanUserBookWithBook(rows)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &ub, nil
}

// FindUserBookByTitleAndAuthor matches title and at least one author exactly,
// case-insensitively.
func (repo *BooksRepository) FindUserBookByTitleAndAuthor(
	ctx context.Context,
	userID string,
	title string,
	author string,
) (*models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1
		  AND lower(b.title) = lower($2)
		  AND EXISTS (
		      SELECT 1 FROM unnest(b.authors) a WHERE lower(a) = lower($3)
		  )
		LIMIT 1
	`

	rows, err := repo.db.Query(ctx, query, userID, title, author)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, database.ErrResourceNotFound
	}

	ub, err := scanUserBookWithBook(rows)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return &ub, nil
}

// DeleteUserBook removes a single user_book row, never the shared catalog row.
func (repo *BooksRepository) DeleteUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `DELETE FROM books.user_books WHERE user_id = $1 AND book_id = $2`
	_, err := repo.db.Exec(ctx, query, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}

// UpdateTags replaces a user_book's tags. koboSync must be true when the tags
// include kobo-sync so kobo_sync_enabled_at is set or preserved; false clears
// it so a re-enable gets a fresh timestamp.
func (repo *BooksRepository) UpdateTags(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	tags []string,
	koboSync bool,
) error {
	query := `
		UPDATE books.user_books
		SET tags = $3,
		    kobo_sync_enabled_at = CASE
		        WHEN $4 THEN COALESCE(kobo_sync_enabled_at, now())
		        ELSE NULL
		    END,
		    updated_at = now()
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := repo.db.Exec(ctx, query, userID, bookID, tags, koboSync)
	return postgres.PgxErrorToHTTPError(err)
}

// UpdateFinishedAt replaces finished_at outright (no COALESCE) so removing a
// date works.
func (repo *BooksRepository) UpdateFinishedAt(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	finishedAt []time.Time,
) error {
	query := `
		UPDATE books.user_books
		SET finished_at = $3,
		    updated_at = now()
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := repo.db.Exec(ctx, query, userID, bookID, finishedAt)
	return postgres.PgxErrorToHTTPError(err)
}

// ListKoboSyncBooks returns the user's kobo-sync books with a ready file:
// "pdf" with the kobo-format-pdf tag, else "kepub".
func (repo *BooksRepository) ListKoboSyncBooks(
	ctx context.Context,
	userID string,
) ([]models.KoboSyncBook, error) {
	query := `
		SELECT b.id, b.title, b.authors, bf.format, bf.storage_key, bf.size_bytes,
		       COALESCE(ub.kobo_sync_enabled_at, ub.added_at), bf.converter_version,
		       ub.kobo_last_synced_converter_version
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		JOIN books.book_files bf
		    ON bf.book_id = ub.book_id
		    AND bf.user_id = ub.user_id
		    AND bf.status = 'ready'
		    AND bf.format = CASE
		        WHEN 'kobo-format-pdf' = ANY(ub.tags) THEN 'pdf'
		        ELSE 'kepub'
		    END
		WHERE ub.user_id = $1 AND 'kobo-sync' = ANY(ub.tags)
		ORDER BY b.title
	`

	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.KoboSyncBook
	for rows.Next() {
		var b models.KoboSyncBook
		if scanErr := rows.Scan(
			&b.BookID, &b.Title, &b.Authors, &b.Format, &b.StorageKey, &b.Size,
			&b.KoboSyncEnabledAt, &b.ConverterVersion, &b.LastSyncedConverterVersion,
		); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, b)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// UpdateKoboLastSyncedConverterVersion records the converter version last sent
// to the device.
func (repo *BooksRepository) UpdateKoboLastSyncedConverterVersion(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	converterVersion int16,
) error {
	query := `
		UPDATE books.user_books
		SET kobo_last_synced_converter_version = $3
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := repo.db.Exec(ctx, query, userID, bookID, converterVersion)
	return postgres.PgxErrorToHTTPError(err)
}

// UpsertKoboRemoval tombstones bookID for removal on the next Kobo sync.
func (repo *BooksRepository) UpsertKoboRemoval(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `
		INSERT INTO books.kobo_removals (user_id, book_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, book_id) DO NOTHING
	`
	_, err := repo.db.Exec(ctx, query, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}

// DeleteKoboRemoval clears a book's removal tombstone.
func (repo *BooksRepository) DeleteKoboRemoval(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `DELETE FROM books.kobo_removals WHERE user_id = $1 AND book_id = $2`
	_, err := repo.db.Exec(ctx, query, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}

// ListKoboRemovals returns books tombstoned for removal from the user's Kobo.
func (repo *BooksRepository) ListKoboRemovals(
	ctx context.Context,
	userID string,
) ([]models.KoboRemoval, error) {
	query := `
		SELECT book_id, removed_at
		FROM books.kobo_removals
		WHERE user_id = $1
	`

	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []models.KoboRemoval
	for rows.Next() {
		var r models.KoboRemoval
		if scanErr := rows.Scan(&r.BookID, &r.RemovedAt); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		out = append(out, r)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return out, nil
}

// ListBooksWithISBN13 returns all catalog books with a non-null ISBN13.
func (repo *BooksRepository) ListBooksWithISBN13(
	ctx context.Context,
) ([]models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		WHERE isbn13 IS NOT NULL
	`

	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		b, scanErr := scanBook(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		books = append(books, *b)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return books, nil
}

// RefreshBookExternalData writes one source's fields as-is, blanking whatever
// it doesn't supply. isbn13 is never blanked and only written when no other
// book holds it, so a fuzzy match can't attach a conflicting ISBN.
func (repo *BooksRepository) RefreshBookExternalData(
	ctx context.Context,
	bookID uuid.UUID,
	coverURL string,
	description string,
	pageCount int,
	isbn13 string,
	title string,
	authors []string,
	metadataSource string,
) error {
	query := `
		UPDATE books.books
		SET cover_url   = NULLIF($2, ''),
		    description = NULLIF($3, ''),
		    page_count  = NULLIF($4, 0),
		    isbn13      = CASE
		                    WHEN $5 <> ''
		                      AND NOT EXISTS (
		                        SELECT 1 FROM books.books
		                        WHERE isbn13 = $5 AND id <> $1
		                      )
		                    THEN $5
		                    ELSE isbn13
		                  END,
		    title       = $6,
		    authors     = $7,
		    metadata_source = NULLIF($8, ''),
		    updated_at  = now()
		WHERE id = $1
	`
	_, err := repo.db.Exec(
		ctx, query,
		bookID, coverURL, description, pageCount, isbn13,
		title, authors, metadataSource,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// UpdateResyncScanStatus records per-source found flags; a nil flag leaves the
// column unchanged (COALESCE).
func (repo *BooksRepository) UpdateResyncScanStatus(
	ctx context.Context,
	bookID uuid.UUID,
	uniCatFound *bool,
	hardcoverFound *bool,
) error {
	query := `
		UPDATE books.books
		SET unicat_found      = COALESCE($2, unicat_found),
		    hardcover_found   = COALESCE($3, hardcover_found),
		    last_resync_at    = now()
		WHERE id = $1
	`
	_, err := repo.db.Exec(
		ctx, query,
		bookID, uniCatFound, hardcoverFound,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// ListCatalogBooks orders catalog books least-covered first, then title, so an
// interrupted or rate-limited resync spends its budget where it's needed.
func (repo *BooksRepository) ListCatalogBooks(
	ctx context.Context,
) ([]models.Book, error) {
	// URL-ingested items have no ISBN and generic titles; scanning them only
	// yields garbage proposals and burns rate limits.
	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		WHERE source_url IS NULL
		ORDER BY (
			(unicat_found IS TRUE)::int +
			(hardcover_found IS TRUE)::int
		), title
	`

	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		b, scanErr := scanBook(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		books = append(books, *b)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return books, nil
}

// sourceColumns maps source names to found columns; SQL column names come only
// from here.
//
//nolint:gochecknoglobals // fixed lookup table, never mutated
var sourceColumns = map[string]string{
	"unicat":    "unicat_found",
	"hardcover": "hardcover_found",
}

// exactSourcesPredicate matches books found (IS TRUE) by exactly the given
// sources and confirmed absent (IS FALSE) from the rest, so an unresolved
// (NULL) source excludes the book. Rejects empty or unknown names.
func exactSourcesPredicate(sources []string) (string, error) {
	if len(sources) == 0 {
		return "", database.ErrResourceNotFound
	}

	want := make(map[string]bool, len(sources))
	for _, s := range sources {
		if _, ok := sourceColumns[s]; !ok {
			return "", database.ErrResourceNotFound
		}
		want[s] = true
	}

	clauses := make([]string, 0, len(sourceColumns))
	for source, column := range sourceColumns {
		if want[source] {
			clauses = append(clauses, column+" IS TRUE")
		} else {
			clauses = append(clauses, column+" IS FALSE")
		}
	}
	sort.Strings(clauses)
	return strings.Join(clauses, " AND "), nil
}

// ListBooksInExactSources returns books found by exactly the given sources,
// ordered by title.
func (repo *BooksRepository) ListBooksInExactSources(
	ctx context.Context,
	sources []string,
) ([]models.Book, error) {
	predicate, err := exactSourcesPredicate(sources)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		WHERE ` + predicate + `
		ORDER BY title
	`

	rows, err := repo.db.Query(ctx, query)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		b, scanErr := scanBook(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		books = append(books, *b)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return books, nil
}

// GetCatalogWithUserOverlay returns every catalog book as a UserBook, with the
// user's own values where they have it in their library.
func (repo *BooksRepository) GetCatalogWithUserOverlay(
	ctx context.Context,
	userID string,
) ([]models.UserBook, error) {
	// Column order must match scanUserBookWithBook.
	query := `
		SELECT
		    COALESCE(ub.id, '00000000-0000-0000-0000-000000000000'::uuid),
		    COALESCE(ub.user_id, ''),
		    b.id,
		    COALESCE(ub.status, ''),
		    ub.tags,
		    ub.shelf_positions,
		    ub.rating,
		    ub.finished_at,
		    COALESCE(ub.progress_mode, ''),
		    COALESCE(ub.current_page, 0),
		    COALESCE(ub.progress_percent, 0),
		    COALESCE(ub.added_at, b.created_at),
		    COALESCE(ub.updated_at, b.updated_at),
		    b.id, b.title, b.authors, b.isbn13, b.cover_url, b.description,
		    b.page_count, b.source_url, b.created_at, b.updated_at,
		    b.content_html IS NOT NULL AND b.content_html <> ''
		FROM books.books b
		LEFT JOIN books.user_books ub
		    ON ub.book_id = b.id AND ub.user_id = $1
		WHERE b.source_url IS NULL
		ORDER BY b.title
	`
	return repo.queryUserBooks(ctx, query, userID)
}

// ListUserBookOwners returns the distinct users owning any of the given books.
func (repo *BooksRepository) ListUserBookOwners(
	ctx context.Context,
	bookIDs []uuid.UUID,
) ([]string, error) {
	if len(bookIDs) == 0 {
		return nil, nil
	}

	strIDs := make([]string, len(bookIDs))
	for i, id := range bookIDs {
		strIDs[i] = id.String()
	}

	query := `
		SELECT DISTINCT user_id
		FROM books.user_books
		WHERE book_id = ANY($1::uuid[])
	`

	rows, err := repo.db.Query(ctx, query, strIDs)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var owners []string
	for rows.Next() {
		var uid string
		if scanErr := rows.Scan(&uid); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		owners = append(owners, uid)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return owners, nil
}

// GetBooksByIDs returns the catalog books with the given IDs, ignoring missing ones.
func (repo *BooksRepository) GetBooksByIDs(
	ctx context.Context,
	ids []uuid.UUID,
) ([]models.Book, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// pgx has no encode plan for []uuid.UUID; pass strings and cast in SQL.
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = id.String()
	}

	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		WHERE id = ANY($1::uuid[])
	`

	rows, err := repo.db.Query(ctx, query, strIDs)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		b, scanErr := scanBook(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		books = append(books, *b)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return books, nil
}

// ListShelves returns every registered custom shelf, including empty ones.
func (repo *BooksRepository) ListShelves(
	ctx context.Context,
	userID string,
) ([]string, error) {
	query := `SELECT name FROM books.shelves WHERE user_id = $1`
	rows, err := repo.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		names = append(names, name)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return names, nil
}

// EnsureShelf registers a custom shelf name so it persists with no books.
// Callers reject built-in statuses.
func (repo *BooksRepository) EnsureShelf(
	ctx context.Context,
	userID string,
	name string,
) error {
	query := `
		INSERT INTO books.shelves (user_id, name)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	_, err := repo.db.Exec(ctx, query, userID, name)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	return nil
}

// RenameShelf moves every book and the registry entry from oldName to newName,
// returning the rows affected. Callers reject built-in statuses.
func (repo *BooksRepository) RenameShelf(
	ctx context.Context,
	userID string,
	oldName string,
	newName string,
) (uint32, error) {
	query := `
		UPDATE books.user_books
		SET status = $3, updated_at = now()
		WHERE user_id = $1 AND status = $2
	`
	tag, err := repo.db.Exec(ctx, query, userID, oldName, newName)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}

	// Delete then insert rather than UPDATE: newName may already be registered,
	// which would violate the (user_id, name) key.
	registryQuery := `
		WITH removed AS (
			DELETE FROM books.shelves WHERE user_id = $1 AND name = $2
		)
		INSERT INTO books.shelves (user_id, name)
		VALUES ($1, $3)
		ON CONFLICT DO NOTHING
	`
	if _, err = repo.db.Exec(ctx, registryQuery, userID, oldName, newName); err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}

	//nolint:gosec // row count is safe for domain values
	return uint32(tag.RowsAffected()), nil
}

// DeleteShelf moves every book on oldName to targetName and unregisters
// oldName, returning the rows moved. Callers reject built-in statuses.
func (repo *BooksRepository) DeleteShelf(
	ctx context.Context,
	userID string,
	oldName string,
	targetName string,
) (uint32, error) {
	query := `
		UPDATE books.user_books
		SET status = $3, updated_at = now()
		WHERE user_id = $1 AND status = $2
	`
	tag, err := repo.db.Exec(ctx, query, userID, oldName, targetName)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}

	deleteQuery := `DELETE FROM books.shelves WHERE user_id = $1 AND name = $2`
	if _, err = repo.db.Exec(ctx, deleteQuery, userID, oldName); err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}

	//nolint:gosec // row count is safe for domain values
	return uint32(tag.RowsAffected()), nil
}

// RenameTag renames a tag across the user's library, returning rows affected.
func (repo *BooksRepository) RenameTag(
	ctx context.Context,
	userID string,
	oldName string,
	newName string,
) (uint32, error) {
	query := `
		UPDATE books.user_books
		SET tags = array_replace(tags, $2, $3), updated_at = now()
		WHERE user_id = $1 AND $2 = ANY(tags)
	`
	tag, err := repo.db.Exec(ctx, query, userID, oldName, newName)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}
	//nolint:gosec // row count is safe for domain values
	return uint32(tag.RowsAffected()), nil
}

// DeleteTag removes a tag across the user's library, returning rows affected.
func (repo *BooksRepository) DeleteTag(
	ctx context.Context,
	userID string,
	name string,
) (uint32, error) {
	query := `
		UPDATE books.user_books
		SET tags = array_remove(tags, $2), updated_at = now()
		WHERE user_id = $1 AND $2 = ANY(tags)
	`
	tag, err := repo.db.Exec(ctx, query, userID, name)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}
	//nolint:gosec // row count is safe for domain values
	return uint32(tag.RowsAffected()), nil
}

// GetKoboSyncBook returns one book under ListKoboSyncBooks' criteria, or
// ErrResourceNotFound.
func (repo *BooksRepository) GetKoboSyncBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (models.KoboSyncBook, error) {
	query := `
		SELECT b.id, b.title, b.authors, bf.format, bf.storage_key, bf.size_bytes,
		       bf.converter_version
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		JOIN books.book_files bf
		    ON bf.book_id = ub.book_id
		    AND bf.user_id = ub.user_id
		    AND bf.status = 'ready'
		    AND bf.format = CASE
		        WHEN 'kobo-format-pdf' = ANY(ub.tags) THEN 'pdf'
		        ELSE 'kepub'
		    END
		WHERE ub.user_id = $1 AND ub.book_id = $2 AND 'kobo-sync' = ANY(ub.tags)
	`

	var b models.KoboSyncBook
	err := repo.db.QueryRow(ctx, query, userID, bookID).Scan(
		&b.BookID, &b.Title, &b.Authors, &b.Format, &b.StorageKey, &b.Size,
		&b.ConverterVersion,
	)
	if err != nil {
		return models.KoboSyncBook{}, postgres.PgxErrorToHTTPError(err)
	}
	return b, nil
}
