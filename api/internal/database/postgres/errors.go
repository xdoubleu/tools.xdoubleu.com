package postgres

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"tools.xdoubleu.com/internal/database"
)

// PgxErrorToHTTPError maps a pgx error to an HTTP error.
func PgxErrorToHTTPError(err error) error {
	var pgxError *pgconn.PgError
	errors.As(err, &pgxError)

	switch {
	case pgxError == nil:
		if errors.Is(err, pgx.ErrNoRows) {
			return database.ErrResourceNotFound
		}
		return err
	case pgxError.Code == pgerrcode.ForeignKeyViolation:
		return database.ErrResourceNotFound
	case pgxError.Code == pgerrcode.UniqueViolation:
		return database.ErrResourceConflict
	default:
		return err
	}
}
