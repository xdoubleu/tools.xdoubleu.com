package repositories

import (
	"context"
	"sort"
	"time"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

// NotificationOutboxRepository stores queued notifications in
// global.notification_outbox.
type NotificationOutboxRepository struct {
	db postgres.DB
}

func NewNotificationOutboxRepository(db postgres.DB) *NotificationOutboxRepository {
	return &NotificationOutboxRepository{db: db}
}

func (r *NotificationOutboxRepository) Enqueue(
	ctx context.Context,
	to, subject, body string,
) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO global.notification_outbox (recipient, subject, body)
		VALUES ($1, $2, $3)
		RETURNING id
	`, to, subject, body).Scan(&id)
	return id, err
}

// ClaimDue returns up to limit pending rows due at now, oldest first, and
// counts an attempt on each. A claimed row is hidden until now+lease, so
// concurrent claimers skip it and a crashed sender's rows come back after.
func (r *NotificationOutboxRepository) ClaimDue(
	ctx context.Context,
	now time.Time,
	lease time.Duration,
	limit int,
) ([]models.OutboxMessage, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE global.notification_outbox
		SET attempts = attempts + 1, next_attempt_at = $2
		WHERE id IN (
			SELECT id FROM global.notification_outbox
			WHERE status = 'pending' AND next_attempt_at <= $1
			ORDER BY id
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, recipient, subject, body, attempts
	`, now, now.Add(lease), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []models.OutboxMessage
	for rows.Next() {
		var m models.OutboxMessage
		if err = rows.Scan(&m.ID, &m.To, &m.Subject, &m.Body, &m.Attempts); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].ID < msgs[j].ID })
	return msgs, rows.Err()
}

func (r *NotificationOutboxRepository) MarkSent(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE global.notification_outbox
		SET status = 'sent', last_error = NULL, finished_at = now()
		WHERE id = $1
	`, id)
	return err
}

// MarkRetry records the error and makes the row due again at next.
func (r *NotificationOutboxRepository) MarkRetry(
	ctx context.Context,
	id int64,
	next time.Time,
	lastErr string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE global.notification_outbox
		SET next_attempt_at = $2, last_error = $3
		WHERE id = $1
	`, id, next, lastErr)
	return err
}

// MarkFailed gives up on the row, keeping the error.
func (r *NotificationOutboxRepository) MarkFailed(
	ctx context.Context,
	id int64,
	lastErr string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE global.notification_outbox
		SET status = 'failed', last_error = $2, finished_at = now()
		WHERE id = $1
	`, id, lastErr)
	return err
}

// Depth counts pending and failed rows.
func (r *NotificationOutboxRepository) Depth(
	ctx context.Context,
) (models.OutboxDepth, error) {
	var d models.OutboxDepth
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE status = 'pending'),
		       COUNT(*) FILTER (WHERE status = 'failed')
		FROM global.notification_outbox
	`).Scan(&d.Pending, &d.Failed)
	return d, err
}

// PruneFinished deletes sent and failed rows finished before the cutoff.
func (r *NotificationOutboxRepository) PruneFinished(
	ctx context.Context,
	before time.Time,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM global.notification_outbox WHERE finished_at < $1`, before)
	return err
}
