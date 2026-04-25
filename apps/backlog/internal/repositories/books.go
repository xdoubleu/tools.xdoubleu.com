package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
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
		    (title, authors, isbn13, isbn10, cover_url, description, external_refs)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL
		DO UPDATE SET
		    title         = EXCLUDED.title,
		    authors       = EXCLUDED.authors,
		    isbn10        = COALESCE(EXCLUDED.isbn10, backlog.books.isbn10),
		    cover_url     = COALESCE(EXCLUDED.cover_url, backlog.books.cover_url),
		    description   = COALESCE(EXCLUDED.description, backlog.books.description),
		    external_refs = backlog.books.external_refs || EXCLUDED.external_refs,
		    updated_at    = now()
		RETURNING id, title, authors, isbn13, isbn10, cover_url, description,
		          external_refs, created_at, updated_at
	`

	row := repo.db.QueryRow(ctx, query,
		book.Title,
		book.Authors,
		book.ISBN13,
		book.ISBN10,
		book.CoverURL,
		book.Description,
		externalRefsJSON,
	)

	return scanBook(row)
}

func (repo *BooksRepository) FindBookByTitleAndAuthor(
	ctx context.Context,
	title string,
	author string,
) (*models.Book, error) {
	query := `
		SELECT id, title, authors, isbn13, isbn10, cover_url, description,
		       external_refs, created_at, updated_at
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
		posJSON,
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
		SELECT ub.id, ub.user_id, ub.book_id, ub.status, ub.tags, ub.shelf_positions,
		       ub.rating, ub.notes, ub.finished_at, ub.added_at, ub.updated_at,
		       b.id, b.title, b.authors, b.isbn13, b.isbn10, b.cover_url,
		       b.description, b.external_refs, b.created_at, b.updated_at
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
		SELECT ub.id, ub.user_id, ub.book_id, ub.status, ub.tags, ub.shelf_positions,
		       ub.rating, ub.notes, ub.finished_at, ub.added_at, ub.updated_at,
		       b.id, b.title, b.authors, b.isbn13, b.isbn10, b.cover_url,
		       b.description, b.external_refs, b.created_at, b.updated_at
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

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func scanBook(row pgx.Row) (*models.Book, error) {
	var book models.Book
	var refsJSON []byte

	err := row.Scan(
		&book.ID,
		&book.Title,
		&book.Authors,
		&book.ISBN13,
		&book.ISBN10,
		&book.CoverURL,
		&book.Description,
		&refsJSON,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	book.ExternalRefs = map[string]string{}
	if len(refsJSON) > 0 {
		if jsonErr := json.Unmarshal(refsJSON, &book.ExternalRefs); jsonErr != nil {
			return nil, jsonErr
		}
	}

	return &book, nil
}

func scanUserBookWithBook(rows pgx.Rows) (models.UserBook, error) {
	var ub models.UserBook
	var book models.Book
	var refsJSON, posJSON []byte

	err := rows.Scan(
		&ub.ID,
		&ub.UserID,
		&ub.BookID,
		&ub.Status,
		&ub.Tags,
		&posJSON,
		&ub.Rating,
		&ub.Notes,
		&ub.FinishedAt,
		&ub.AddedAt,
		&ub.UpdatedAt,
		&book.ID,
		&book.Title,
		&book.Authors,
		&book.ISBN13,
		&book.ISBN10,
		&book.CoverURL,
		&book.Description,
		&refsJSON,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		return models.UserBook{}, err
	}

	book.ExternalRefs = map[string]string{}
	if len(refsJSON) > 0 {
		if jsonErr := json.Unmarshal(refsJSON, &book.ExternalRefs); jsonErr != nil {
			return models.UserBook{}, jsonErr
		}
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
		    (title, authors, isbn13, isbn10, cover_url, description, external_refs)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL
		DO UPDATE SET
		    title         = EXCLUDED.title,
		    authors       = EXCLUDED.authors,
		    cover_url     = COALESCE(EXCLUDED.cover_url, backlog.books.cover_url),
		    description   = COALESCE(EXCLUDED.description, backlog.books.description),
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
		SELECT id, title, authors, isbn13, isbn10, cover_url, description,
		       external_refs, created_at, updated_at
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
		SELECT ub.id, ub.user_id, ub.book_id, ub.status, ub.tags, ub.shelf_positions,
		       ub.rating, ub.notes, ub.finished_at, ub.added_at, ub.updated_at,
		       b.id, b.title, b.authors, b.isbn13, b.isbn10, b.cover_url,
		       b.description, b.external_refs, b.created_at, b.updated_at
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
		return nil, nil //nolint:nilnil //caller uses nil check for not-found
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
		SELECT ub.id, ub.user_id, ub.book_id, ub.status, ub.tags, ub.shelf_positions,
		       ub.rating, ub.notes, ub.finished_at, ub.added_at, ub.updated_at,
		       b.id, b.title, b.authors, b.isbn13, b.isbn10, b.cover_url,
		       b.description, b.external_refs, b.created_at, b.updated_at
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

// UpdateTags replaces the tag list for a user_book.
func (repo *BooksRepository) UpdateTags(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	tags []string,
) error {
	query := `
		UPDATE backlog.user_books
		SET tags = $3, updated_at = now()
		WHERE user_id = $1 AND book_id = $2
	`
	_, err := repo.db.Exec(ctx, query, userID, bookID, tags)
	return postgres.PgxErrorToHTTPError(err)
}
