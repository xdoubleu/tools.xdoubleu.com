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
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/books/internal/models"
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
		     metadata_source)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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

// GetBookByID returns the book with the given ID.
// Returns database.ErrResourceNotFound when no book matches.
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

// GetCatalogBookByISBN13 returns the catalog book with the given ISBN-13.
// Returns database.ErrResourceNotFound when no book matches.
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

// UpdateBookByID overwrites the catalog fields of an existing book row, matched
// strictly by its primary key. Unlike UpsertBook this never matches on the
// isbn13 unique index, so it is safe to use when the resolved ISBN differs from
// the current winner's ISBN.
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
	)

	return postgres.PgxErrorToHTTPError(err)
}

// DeleteOrphanedBook deletes a catalog book row only when no user_books row
// still references it. The returned bool reports whether a row was actually
// deleted, so callers know whether to also clean up the book's R2 objects.
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
		SELECT UNNEST(finished_at) AS finished_date
		FROM books.user_books
		WHERE user_id = $1 AND status = 'read'
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

	// 🔒 Hard guard: must align
	if len(books) != len(userBooks) {
		return fmt.Errorf(
			"books and userBooks length mismatch: %d vs %d",
			len(books),
			len(userBooks),
		)
	}

	// ---------------------------
	// 1. UPSERT BOOKS
	// ---------------------------
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

	// ---------------------------
	// 2. ASSIGN BOOK IDs
	// ---------------------------
	for i := range userBooks {
		userBooks[i].BookID = uuid.MustParse(bookIDs[i])
	}

	// ---------------------------
	// 3. UPSERT USER_BOOKS
	// ---------------------------
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

// SearchLibrary does a case-insensitive substring search across the user's own books.
func (repo *BooksRepository) SearchLibrary(
	ctx context.Context,
	userID string,
	query string,
) ([]models.UserBook, error) {
	q := `
		SELECT ` + userBookColumns + `
		FROM books.user_books ub
		JOIN books.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1
		  AND (
		        b.title ILIKE '%' || $2 || '%'
		        OR EXISTS (
		            SELECT 1 FROM UNNEST(b.authors) a WHERE a ILIKE '%' || $2 || '%'
		        )
		  )
		ORDER BY b.title
	`

	return repo.queryUserBooks(ctx, q, userID, query)
}

// FindUserBookByISBN13 finds the user's library entry for a book with the given ISBN13.
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

// FindUserBookByTitleAndAuthor finds a user_book using case-insensitive exact
// matching on title and at least one author.
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

// DeleteUserBook removes a single user_book row.
// It does NOT touch books.books (the shared catalog).
func (repo *BooksRepository) DeleteUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `DELETE FROM books.user_books WHERE user_id = $1 AND book_id = $2`
	_, err := repo.db.Exec(ctx, query, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}

// UpdateTags replaces the tag list for a user_book.
// koboSync must be true when the resulting tag list contains the kobo-sync
// tag so that kobo_sync_enabled_at is set (on first enable) or preserved
// (when other tags change while kobo-sync stays). Passing false clears the
// column so a re-enable gets a fresh timestamp.
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

// UpdateFinishedAt overwrites a user_book's finished_at date array. Unlike
// UpsertUserBook, this always replaces the array (no COALESCE) so removing a
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

// ListKoboSyncBooks returns all books for a user that have the kobo-sync tag
// and a ready file to serve to the Kobo device. The file format is chosen
// per-book: "pdf" when the kobo-format-pdf tag is present, "kepub" otherwise.
func (repo *BooksRepository) ListKoboSyncBooks(
	ctx context.Context,
	userID string,
) ([]models.KoboSyncBook, error) {
	query := `
		SELECT b.id, b.title, b.authors, bf.format, bf.storage_key, bf.size_bytes,
		       COALESCE(ub.kobo_sync_enabled_at, ub.added_at)
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
			&b.KoboSyncEnabledAt,
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

// ListBooksWithISBN13 returns all catalog books that have a non-null ISBN13.
// Kept for backward compatibility; prefer ListBooksMissingMetadata for resync.
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

// RefreshBookExternalData backfills a book's externally-sourced fields.
// All columns use COALESCE so a nil argument never erases an existing value —
// callers pass nil for fields they do not want to touch.
//
// The isbn13 update is guarded by a NOT EXISTS subquery: if another book in
// the catalog already has that ISBN the write is silently skipped, preventing
// a unique-constraint error from a fuzzy title/author match attaching the
// wrong ISBN.
func (repo *BooksRepository) RefreshBookExternalData(
	ctx context.Context,
	bookID uuid.UUID,
	coverURL *string,
	description *string,
	pageCount *int,
	isbn13 *string,
	title *string,
	authors []string,
	metadataSource string,
) error {
	// authors is passed as a Go nil slice when the caller has nothing to write;
	// convert to a typed nil so COALESCE sees a true SQL NULL rather than an
	// empty array, which would overwrite the existing value.
	var authorsArg *[]string
	if len(authors) > 0 {
		authorsArg = &authors
	}

	query := `
		UPDATE books.books
		SET cover_url   = COALESCE($2, cover_url),
		    description = COALESCE($3, description),
		    page_count  = COALESCE($4, page_count),
		    isbn13      = COALESCE(
		                    CASE
		                      WHEN $5::text IS NOT NULL
		                        AND NOT EXISTS (
		                          SELECT 1 FROM books.books
		                          WHERE isbn13 = $5 AND id <> $1
		                        )
		                      THEN $5::text
		                    END,
		                    isbn13
		                  ),
		    title       = COALESCE($6, title),
		    authors     = COALESCE($7, authors),
		    metadata_source = NULLIF($8, ''),
		    updated_at  = now()
		WHERE id = $1
	`
	_, err := repo.db.Exec(
		ctx, query,
		bookID, coverURL, description, pageCount, isbn13,
		title, authorsArg, metadataSource,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// UpdateResyncScanStatus records one scan pass's per-source found flags and
// bumps last_resync_at. Nil flags mean "provider not configured" or "book not
// searchable" and write NULL.
// UpdateResyncScanStatus records one scan pass's per-source found flags. A
// nil flag means the source wasn't resolved this pass — not configured, not
// attempted, skipped because already known, or its call errored (including a
// throttled Google Books lookup) — and must leave the column unchanged
// (COALESCE) rather than overwrite a previously-known value with NULL/false.
// Only a source that was actually queried and answered this pass writes a
// fresh true/false.
func (repo *BooksRepository) UpdateResyncScanStatus(
	ctx context.Context,
	bookID uuid.UUID,
	openLibraryFound *bool,
	googleBooksFound *bool,
	uniCatFound *bool,
) error {
	query := `
		UPDATE books.books
		SET openlibrary_found = COALESCE($2, openlibrary_found),
		    googlebooks_found = COALESCE($3, googlebooks_found),
		    unicat_found      = COALESCE($4, unicat_found),
		    last_resync_at    = now()
		WHERE id = $1
	`
	_, err := repo.db.Exec(
		ctx, query,
		bookID, openLibraryFound, googleBooksFound, uniCatFound,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// ListCatalogBooks returns all catalog books ordered least-covered-first (by
// count of sources with a confirmed found = true), then title. Used by the
// admin resync wizard scan: Google Books has a ~1000/day free-tier quota, so
// once a full-catalog force resync trips its circuit breaker (see
// BuildResyncProposals), whatever books are left in the scan order never get
// a GB check that run. Alphabetical order let that quota starve the tail of
// the catalog; ordering by coverage gap instead spends the budget on the
// books that most need it — never-scanned and not-yet-found books sort
// first, already-well-covered books last.
func (repo *BooksRepository) ListCatalogBooks(
	ctx context.Context,
) ([]models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM books.books
		ORDER BY (
			(openlibrary_found IS TRUE)::int +
			(googlebooks_found IS TRUE)::int +
			(unicat_found IS TRUE)::int
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

// sourceColumns maps a source name to its found column — the fixed,
// known set of GetSourceStats sources. Never build SQL from unvalidated
// input directly; exactSourcesPredicate only emits column names from here.
//
//nolint:gochecknoglobals // fixed lookup table, never mutated
var sourceColumns = map[string]string{
	"openlibrary": "openlibrary_found",
	"googlebooks": "googlebooks_found",
	"unicat":      "unicat_found",
}

// exactSourcesPredicate returns the SQL boolean expression matching books
// found by exactly the given set of sources — found (IS TRUE) for each named
// source, confirmed absent (IS FALSE) for every other known source. A source
// still unknown (NULL) never satisfies IS FALSE, so an unresolved source
// correctly excludes a book from any exact-set match (see SourceStats' doc).
// Rejects an empty set or any name outside sourceColumns.
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

// ListBooksInExactSources returns the catalog books found by exactly the
// given set of sources (a single source is GetSourceStats' *Unique books; two
// or three sources is an overlap combo), ordered by title.
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

// GetCatalogWithUserOverlay returns all catalog books as UserBook entries.
// Books that the given user has added to their library carry their real
// user_book values; books not in the library have empty status/tags/etc.
// This is used by catalog-wide duplicate detection (FindDuplicates).
func (repo *BooksRepository) GetCatalogWithUserOverlay(
	ctx context.Context,
	userID string,
) ([]models.UserBook, error) {
	// Column order and types must match scanUserBookWithBook. COALESCE provides
	// zero-like defaults for ub columns that are NULL when the user has not
	// added a catalog book to their library.
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
		    b.page_count, b.created_at, b.updated_at
		FROM books.books b
		LEFT JOIN books.user_books ub
		    ON ub.book_id = b.id AND ub.user_id = $1
		ORDER BY b.title
	`
	return repo.queryUserBooks(ctx, query, userID)
}

// ListUserBookOwners returns the distinct user_ids that have a user_books row
// for any of the given book IDs. Used by the global merge to discover all
// affected users before iterating.
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

// GetBooksByIDs returns the catalog books whose IDs are in the given slice.
// Missing IDs are silently ignored.
func (repo *BooksRepository) GetBooksByIDs(
	ctx context.Context,
	ids []uuid.UUID,
) ([]models.Book, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// pgx has no encode plan for []uuid.UUID; convert to []string and cast in
	// SQL so Postgres knows the element type.
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

// ListShelves returns every custom shelf name registered for the user,
// including shelves with zero books on them.
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

// EnsureShelf registers a custom shelf name for the user if it isn't already
// registered, so it persists in ListShelves even once it has no books left.
// The caller is responsible for rejecting built-in status values.
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

// RenameShelf updates the status of every user_book with status == oldName to
// newName, and moves the shelf's registry entry along with it. Returns the
// number of user_books rows affected.
// The caller is responsible for rejecting built-in status values.
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

	// Drop the old registry entry and (re-)register the new name in one
	// statement, rather than UPDATE-ing the row in place: newName might
	// already be registered (e.g. an empty shelf someone created
	// separately), and an in-place UPDATE would violate the (user_id, name)
	// primary key in that case. Deleting then inserting merges into the
	// existing entry instead of erroring.
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

// DeleteShelf reassigns every book on shelf oldName to targetName and removes
// oldName from the shelf registry. Returns the number of rows moved.
// The caller is responsible for rejecting built-in status values.
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

// RenameTag replaces every occurrence of oldName in the tags array with
// newName across the user's library. Returns the number of rows affected.
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

// DeleteTag removes every occurrence of name from the tags array across the
// user's library. Returns the number of rows affected.
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

// GetKoboSyncBook returns the single kobo-sync book matching bookID for the
// user. It uses the same eligibility criteria as ListKoboSyncBooks: the book
// must have the kobo-sync tag and a ready file. Returns
// database.ErrResourceNotFound when no matching row exists.
func (repo *BooksRepository) GetKoboSyncBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (models.KoboSyncBook, error) {
	query := `
		SELECT b.id, b.title, b.authors, bf.format, bf.storage_key, bf.size_bytes
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
	)
	if err != nil {
		return models.KoboSyncBook{}, postgres.PgxErrorToHTTPError(err)
	}
	return b, nil
}
