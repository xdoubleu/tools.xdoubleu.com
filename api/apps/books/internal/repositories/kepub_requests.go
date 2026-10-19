package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/database/postgres"
)

// KEPUBRequest is a pending KEPUB conversion.
type KEPUBRequest struct {
	UserID string
	BookID uuid.UUID
}

// KEPUBRequestsRepository is the queue the web role fills and the workers
// role drains.
type KEPUBRequestsRepository struct {
	db postgres.DB
}

// Enqueue records a pending conversion; an already pending one is kept.
func (r *KEPUBRequestsRepository) Enqueue(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO books.kepub_conversion_requests (user_id, book_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}

// ListOldest returns up to limit pending conversions, oldest first.
func (r *KEPUBRequestsRepository) ListOldest(
	ctx context.Context,
	limit int,
) ([]KEPUBRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, book_id
		FROM books.kepub_conversion_requests
		ORDER BY requested_at
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var out []KEPUBRequest
	for rows.Next() {
		var req KEPUBRequest
		if err = rows.Scan(&req.UserID, &req.BookID); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, req)
	}
	return out, postgres.PgxErrorToHTTPError(rows.Err())
}

// Delete removes a conversion request once it has been attempted.
func (r *KEPUBRequestsRepository) Delete(
	ctx context.Context,
	userID string,
	bookID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM books.kepub_conversion_requests
		WHERE user_id = $1 AND book_id = $2
	`, userID, bookID)
	return postgres.PgxErrorToHTTPError(err)
}
