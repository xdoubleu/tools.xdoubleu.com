package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

type BooksRepository struct {
	db postgres.DB
}

func (repo *BooksRepository) UpsertBook(
	ctx context.Context,
	book models.Book,
) (*models.Book, error) {
	externalRefsJSON, err := json.Marshal(book.ExternalRefs)
	if err != nil {
		return nil, err
	}

	// Try match by ISBN13 first, then fall back to title+first author.
	query := `
		INSERT INTO backlog.books
		    (title, authors, isbn13, isbn10, cover_url, description,
		     page_count, external_refs)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL
		DO UPDATE SET
		    title         = EXCLUDED.title,
		    authors       = EXCLUDED.authors,
		    isbn10        = COALESCE(EXCLUDED.isbn10, backlog.books.isbn10),
		    cover_url     = COALESCE(EXCLUDED.cover_url, backlog.books.cover_url),
		    description   = COALESCE(EXCLUDED.description, backlog.books.description),
		    page_count    = COALESCE(EXCLUDED.page_count, backlog.books.page_count),
		    external_refs = backlog.books.external_refs || EXCLUDED.external_refs,
		    updated_at    = now()
		RETURNING ` + bookColumns

	row := repo.db.QueryRow(ctx, query,
		book.Title,
		book.Authors,
		book.ISBN13,
		book.ISBN10,
		book.CoverURL,
		book.Description,
		book.PageCount,
		string(externalRefsJSON),
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
		FROM backlog.books
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
		FROM backlog.books
		WHERE id = $1
	`

	row := repo.db.QueryRow(ctx, query, bookID)
	book, err := scanBook(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return book, nil
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
		INSERT INTO backlog.user_books
		    (user_id, book_id, status, tags, shelf_positions,
		     rating, notes, finished_at, added_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, now()))
		ON CONFLICT (user_id, book_id) DO UPDATE SET
		    status          = EXCLUDED.status,
		    tags            = EXCLUDED.tags,
		    shelf_positions = EXCLUDED.shelf_positions,
		    rating          = COALESCE(EXCLUDED.rating, backlog.user_books.rating),
		    notes           = COALESCE(EXCLUDED.notes, backlog.user_books.notes),
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
		ub.Notes,
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
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
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
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
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
		FROM backlog.user_books
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
		INSERT INTO backlog.books
		    (title, authors, isbn13, isbn10, cover_url, description,
		     page_count, external_refs)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL
		DO UPDATE SET
		    title         = EXCLUDED.title,
		    authors       = EXCLUDED.authors,
		    cover_url     = COALESCE(EXCLUDED.cover_url, backlog.books.cover_url),
		    description   = COALESCE(EXCLUDED.description, backlog.books.description),
		    page_count    = COALESCE(EXCLUDED.page_count, backlog.books.page_count),
		    external_refs = backlog.books.external_refs
		              || COALESCE(EXCLUDED.external_refs, '{}'),
		    updated_at    = now()
		RETURNING id
	`

	bookIDs := make([]string, len(books))

	batch := &pgx.Batch{} //nolint:exhaustruct //QueuedQueries populated via Queue()

	for _, book := range books {
		refsJSON, err := json.Marshal(book.ExternalRefs)
		if err != nil {
			return fmt.Errorf("marshal external refs: %w", err)
		}

		batch.Queue(
			upsertBookQuery,
			book.Title,
			book.Authors,
			book.ISBN13,
			book.ISBN10,
			book.CoverURL,
			book.Description,
			book.PageCount,
			string(refsJSON),
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
		INSERT INTO backlog.user_books
		    (user_id, book_id, status, tags, shelf_positions,
		     rating, notes, finished_at, added_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, now()))
		ON CONFLICT (user_id, book_id) DO UPDATE SET
		    status          = EXCLUDED.status,
		    tags            = EXCLUDED.tags,
		    shelf_positions = EXCLUDED.shelf_positions,
		    rating          = COALESCE(EXCLUDED.rating, backlog.user_books.rating),
		    finished_at     = EXCLUDED.finished_at,
		    added_at        = COALESCE(backlog.user_books.added_at, EXCLUDED.added_at),
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
			ub.Notes,
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

// FindByExternalRef returns a book by provider and provider ID stored in external_refs.
func (repo *BooksRepository) FindByExternalRef(
	ctx context.Context,
	provider string,
	providerID string,
) (*models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM backlog.books
		WHERE external_refs->>$1 = $2
		LIMIT 1
	`

	row := repo.db.QueryRow(ctx, query, provider, providerID)
	book, err := scanBook(row)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return book, nil
}

// GetUserBook fetches a single user_book by user and book ID.
func (repo *BooksRepository) GetUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (*models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
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
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
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
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
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

// FindUserBookByISBN10 finds the user's library entry for a book with the
// given ISBN10.
func (repo *BooksRepository) FindUserBookByISBN10(
	ctx context.Context,
	userID string,
	isbn10 string,
) (*models.UserBook, error) {
	query := `
		SELECT ` + userBookColumns + `
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
		WHERE ub.user_id = $1 AND b.isbn10 = $2
		LIMIT 1
	`

	rows, err := repo.db.Query(ctx, query, userID, isbn10)
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
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
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
// It does NOT touch backlog.books (the shared catalog).
func (repo *BooksRepository) DeleteUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	query := `DELETE FROM backlog.user_books WHERE user_id = $1 AND book_id = $2`
	_, err := repo.db.Exec(ctx, query, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}

// DeleteUserBooks removes all entries from user_books for a given user.
// It does NOT touch backlog.books (the shared catalog).
func (repo *BooksRepository) DeleteUserBooks(
	ctx context.Context,
	userID string,
) (int64, error) {
	query := `DELETE FROM backlog.user_books WHERE user_id = $1`
	tag, err := repo.db.Exec(ctx, query, userID)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}
	return tag.RowsAffected(), nil
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
		UPDATE backlog.user_books
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
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
		JOIN backlog.book_files bf
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
// Used by the Open Library resync job to re-fetch metadata and covers.
func (repo *BooksRepository) ListBooksWithISBN13(
	ctx context.Context,
) ([]models.Book, error) {
	query := `
		SELECT ` + bookColumns + `
		FROM backlog.books
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

// RefreshBookExternalData updates a book's Open Library-sourced fields.
// cover_url is always overwritten; description and page_count use COALESCE so
// a nil from Open Library never erases data we already have.
func (repo *BooksRepository) RefreshBookExternalData(
	ctx context.Context,
	bookID uuid.UUID,
	coverURL *string,
	description *string,
	pageCount *int,
) error {
	query := `
		UPDATE backlog.books
		SET cover_url   = $2,
		    description = COALESCE($3, description),
		    page_count  = COALESCE($4, page_count),
		    updated_at  = now()
		WHERE id = $1
	`
	_, err := repo.db.Exec(ctx, query, bookID, coverURL, description, pageCount)
	return postgres.PgxErrorToHTTPError(err)
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
		FROM backlog.user_books ub
		JOIN backlog.books b ON b.id = ub.book_id
		JOIN backlog.book_files bf
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
