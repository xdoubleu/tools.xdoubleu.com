package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

const bookFileColumns = `id, book_id, user_id, format, storage_key, size_bytes,
	checksum, original_filename, status, source_file_id, created_at, updated_at`

type BookFilesRepository struct {
	db postgres.DB
}

func (r *BookFilesRepository) Insert(
	ctx context.Context,
	f models.BookFile,
) (*models.BookFile, error) {
	query := `
		INSERT INTO backlog.book_files
		    (book_id, user_id, format, storage_key, size_bytes,
		     checksum, original_filename, status, source_file_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
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
	)

	return scanBookFile(row)
}

func (r *BookFilesRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM backlog.book_files
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
		FROM backlog.book_files
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
		FROM backlog.book_files
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
		UPDATE backlog.book_files
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
	query := `DELETE FROM backlog.book_files WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return postgres.PgxErrorToHTTPError(err)
}

func (r *BookFilesRepository) StorageKeysByUser(
	ctx context.Context,
	userID string,
) ([]string, error) {
	query := `SELECT storage_key FROM backlog.book_files WHERE user_id = $1`

	rows, err := r.db.Query(ctx, query, userID)
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

func (r *BookFilesRepository) DeleteByUser(
	ctx context.Context,
	userID string,
) (int64, error) {
	query := `DELETE FROM backlog.book_files WHERE user_id = $1`
	tag, err := r.db.Exec(ctx, query, userID)
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
) error {
	query := `
		UPDATE backlog.book_files
		SET storage_key = $2, size_bytes = $3, status = $4, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, storageKey, sizeBytes, models.FileStatusReady)
	return postgres.PgxErrorToHTTPError(err)
}

// FindByChecksumGlobal returns any book_files row with the given checksum,
// regardless of user or book. Used for global content-addressed deduplication.
// Returns database.ErrResourceNotFound when no row matches.
func (r *BookFilesRepository) FindByChecksumGlobal(
	ctx context.Context,
	checksum string,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM backlog.book_files
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

// FindByStorageKeyGlobal returns any ready book_files row with the given
// storage key, regardless of user or book. Used for cross-user KEPUB
// deduplication: if another user already converted the same source, their
// canonical blob can be reused. Returns database.ErrResourceNotFound when no
// ready row matches.
func (r *BookFilesRepository) FindByStorageKeyGlobal(
	ctx context.Context,
	storageKey string,
) (*models.BookFile, error) {
	query := `
		SELECT ` + bookFileColumns + `
		FROM backlog.book_files
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

// CountByStorageKey returns the number of book_files rows that reference the
// given storage key. Used for refcount-safe deletion: only delete the R2
// object when this count drops to 0.
func (r *BookFilesRepository) CountByStorageKey(
	ctx context.Context,
	storageKey string,
) (int64, error) {
	query := `
		SELECT count(*)
		FROM backlog.book_files
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
		FROM backlog.book_files
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

// FormatsByUser returns a map of book ID → sorted list of ready file formats
// (pdf, epub only — kepub is excluded) for all of a user's books in one query.
func (r *BookFilesRepository) FormatsByUser(
	ctx context.Context,
	userID string,
) (map[uuid.UUID][]string, error) {
	query := `
		SELECT book_id, array_agg(DISTINCT format ORDER BY format)
		FROM backlog.book_files
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
		&f.CreatedAt,
		&f.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &f, nil
}
