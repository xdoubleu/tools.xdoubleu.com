package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/books/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
)

const bookFileColumns = `id, book_id, user_id, format, storage_key, size_bytes,
	checksum, original_filename, status, source_file_id, converter_version,
	created_at, updated_at`

type BookFilesRepository struct {
	db postgres.DB
}

func (r *BookFilesRepository) Insert(
	ctx context.Context,
	f models.BookFile,
) (*models.BookFile, error) {
	query := `
		INSERT INTO books.book_files
		    (book_id, user_id, format, storage_key, size_bytes,
		     checksum, original_filename, status, source_file_id, converter_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + bookFileColumns

	row := r.db.QueryRow(ctx, query,
		f.BookID,
		f.UserID,
		f.Format,
		f.StorageKey,
		f.SizeBytes,
		f.Checksum,
		f.OriginalFilename,
		f.Status,
		f.SourceFileID,
		f.ConverterVersion,
	)

	return scanBookFile(row)
}

func (r *BookFilesRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM books.book_files
		WHERE id = $1
	`

	row := r.db.QueryRow(ctx, query, id)
	f, err := scanBookFile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, database.ErrResourceNotFound
		}
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return f, nil
}

func (r *BookFilesRepository) ListByBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) ([]models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM books.book_files
		WHERE user_id = $1 AND book_id = $2
		ORDER BY created_at
	`

	rows, err := r.db.Query(ctx, query, userID, bookID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var files []models.BookFile
	for rows.Next() {
		f, scanErr := scanBookFile(rows)
		if scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		files = append(files, *f)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return files, nil
}

func (r *BookFilesRepository) GetByBookAndFormat(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	format string,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM books.book_files
		WHERE user_id = $1 AND book_id = $2 AND format = $3
		ORDER BY created_at
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query, userID, bookID, format)
	f, err := scanBookFile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, database.ErrResourceNotFound
		}
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return f, nil
}

func (r *BookFilesRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status string,
) error {
	query := `
		UPDATE books.book_files
		SET status = $2, updated_at = now()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id, status)
	return postgres.PgxErrorToHTTPError(err)
}

func (r *BookFilesRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	query := `DELETE FROM books.book_files WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return postgres.PgxErrorToHTTPError(err)
}

// AllStorageKeys returns every storage key referenced by any book file.
func (r *BookFilesRepository) AllStorageKeys(
	ctx context.Context,
) ([]string, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT DISTINCT storage_key FROM books.book_files`,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if scanErr := rows.Scan(&key); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		keys = append(keys, key)
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return keys, nil
}

// DeleteByUserBook removes a user's files for a single book.
func (r *BookFilesRepository) DeleteByUserBook(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) (int64, error) {
	query := `DELETE FROM books.book_files WHERE user_id = $1 AND book_id = $2`
	tag, err := r.db.Exec(ctx, query, userID, bookID)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}
	return tag.RowsAffected(), nil
}

func (r *BookFilesRepository) UpdateAfterConversion(
	ctx context.Context,
	id uuid.UUID,
	storageKey string,
	sizeBytes int64,
	converterVersion int16,
) error {
	query := `
		UPDATE books.book_files
		SET storage_key = $2, size_bytes = $3, status = $4,
		    converter_version = $5, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.Exec(
		ctx, query, id, storageKey, sizeBytes, models.FileStatusReady, converterVersion,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// FindByChecksumGlobal returns any row with the checksum, or
// ErrResourceNotFound.
func (r *BookFilesRepository) FindByChecksumGlobal(
	ctx context.Context,
	checksum string,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM books.book_files
		WHERE checksum = $1
		LIMIT 1
	`

	rows, err := r.db.Query(ctx, query, checksum)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, database.ErrResourceNotFound
	}

	f, err := scanBookFile(rows)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return f, nil
}

// FindByStorageKeyGlobal returns any ready row with the key, for cross-user
// KEPUB reuse, or ErrResourceNotFound.
func (r *BookFilesRepository) FindByStorageKeyGlobal(
	ctx context.Context,
	storageKey string,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM books.book_files
		WHERE storage_key = $1 AND status = 'ready'
		LIMIT 1
	`

	rows, err := r.db.Query(ctx, query, storageKey)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, database.ErrResourceNotFound
	}

	f, err := scanBookFile(rows)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return f, nil
}

// CountByStorageKey counts rows referencing key; delete the R2 object only at 0.
func (r *BookFilesRepository) CountByStorageKey(
	ctx context.Context,
	storageKey string,
) (int64, error) {
	query := `
		SELECT count(*)
		FROM books.book_files
		WHERE storage_key = $1
	`

	var n int64
	err := r.db.QueryRow(ctx, query, storageKey).Scan(&n)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}

	return n, nil
}

func (r *BookFilesRepository) FindByChecksum(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
	format string,
	checksum string,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM books.book_files
		WHERE user_id = $1 AND book_id = $2 AND format = $3 AND checksum = $4
		LIMIT 1
	`

	rows, err := r.db.Query(ctx, query, userID, bookID, format, checksum)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, database.ErrResourceNotFound
	}

	f, err := scanBookFile(rows)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return f, nil
}

// FormatsByUser returns book ID -> sorted ready formats (pdf, epub; no kepub).
func (r *BookFilesRepository) FormatsByUser(
	ctx context.Context,
	userID string,
) (map[uuid.UUID][]string, error) {
	query := `
		SELECT book_id, array_agg(DISTINCT format ORDER BY format)
		FROM books.book_files
		WHERE user_id = $1
		  AND status = 'ready'
		  AND format IN ('pdf', 'epub')
		GROUP BY book_id
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]string)
	for rows.Next() {
		var bookID uuid.UUID
		var formats []string
		if scanErr := rows.Scan(&bookID, &formats); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		result[bookID] = formats
	}

	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return result, nil
}

// RepointAndDedup moves a user's files from fromBookID to toBookID, deleting
// exact (format, checksum) duplicates and returning their storage keys for R2
// cleanup.
func (r *BookFilesRepository) RepointAndDedup(
	ctx context.Context,
	userID string,
	fromBookID uuid.UUID,
	toBookID uuid.UUID,
) ([]string, error) {
	deleteQuery := `
		DELETE FROM books.book_files f
		USING (
			SELECT format, checksum
			FROM books.book_files
			WHERE user_id = $1 AND book_id = $3
		) winner
		WHERE f.user_id = $1
		  AND f.book_id = $2
		  AND f.format   = winner.format
		  AND f.checksum = winner.checksum
		RETURNING f.storage_key
	`

	rows, err := r.db.Query(ctx, deleteQuery, userID, fromBookID, toBookID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if scanErr := rows.Scan(&key); scanErr != nil {
			return nil, postgres.PgxErrorToHTTPError(scanErr)
		}
		keys = append(keys, key)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	rows.Close()

	repointQuery := `
		UPDATE books.book_files
		SET book_id = $3, updated_at = now()
		WHERE user_id = $1 AND book_id = $2
	`
	_, err = r.db.Exec(ctx, repointQuery, userID, fromBookID, toBookID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	return keys, nil
}

func scanBookFile(row pgx.Row) (*models.BookFile, error) {
	var f models.BookFile

	err := row.Scan(
		&f.ID,
		&f.BookID,
		&f.UserID,
		&f.Format,
		&f.StorageKey,
		&f.SizeBytes,
		&f.Checksum,
		&f.OriginalFilename,
		&f.Status,
		&f.SourceFileID,
		&f.ConverterVersion,
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &f, nil
}
