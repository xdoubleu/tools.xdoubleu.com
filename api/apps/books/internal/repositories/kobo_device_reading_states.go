package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/database/postgres"
)

// ListKoboDeviceHeld returns, per book, the reading-state updated_at that
// deviceID last holds.
func (r *BookReadingStateRepository) ListKoboDeviceHeld(
	ctx context.Context,
	deviceID string,
) (map[uuid.UUID]time.Time, error) {
	rows, err := r.db.Query(ctx, `
		SELECT book_id, state_updated_at
		FROM books.kobo_device_reading_states
		WHERE device_id = $1
	`, deviceID)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	held := map[uuid.UUID]time.Time{}
	for rows.Next() {
		var bookID uuid.UUID
		var at time.Time
		if err = rows.Scan(&bookID, &at); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		held[bookID] = at
	}
	return held, postgres.PgxErrorToHTTPError(rows.Err())
}

// MarkKoboDeviceHeld records that deviceID holds each book's state as of the
// matching updatedAt. A held timestamp never moves backwards.
func (r *BookReadingStateRepository) MarkKoboDeviceHeld(
	ctx context.Context,
	deviceID string,
	bookIDs []uuid.UUID,
	updatedAts []time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO books.kobo_device_reading_states
		    (device_id, book_id, state_updated_at)
		SELECT $1::uuid, b.book_id, b.state_updated_at
		FROM unnest($2::uuid[], $3::timestamptz[]) AS b (book_id, state_updated_at)
		ON CONFLICT (device_id, book_id) DO UPDATE
		    SET state_updated_at = GREATEST(
		        books.kobo_device_reading_states.state_updated_at,
		        EXCLUDED.state_updated_at
		    )
	`, deviceID, bookIDs, updatedAts)
	return postgres.PgxErrorToHTTPError(err)
}
