package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

type JobEventsRepository struct {
	db postgres.DB
}

// Append stores events under the seq the runner gave them; one the job already
// has is ignored, so a retried batch adds nothing twice.
func (r *JobEventsRepository) Append(
	ctx context.Context,
	jobID uuid.UUID,
	seqs []int32,
	kinds, payloads []string,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO factory.job_events (job_id, seq, kind, payload)
		SELECT $1, t.seq, t.kind, t.payload
		FROM unnest($2::int[], $3::text[], $4::text[]) AS t(seq, kind, payload)
		ON CONFLICT (job_id, seq) DO NOTHING`, jobID, seqs, kinds, payloads)
	return postgres.PgxErrorToHTTPError(err)
}

// After lists a job's events with seq above afterSeq, oldest first.
func (r *JobEventsRepository) After(
	ctx context.Context,
	jobID uuid.UUID,
	afterSeq, limit int,
) ([]models.JobEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT job_id, seq, kind, payload, created_at
		FROM factory.job_events
		WHERE job_id = $1 AND seq > $2
		ORDER BY seq LIMIT $3`, jobID, afterSeq, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := []models.JobEvent{}
	for rows.Next() {
		var e models.JobEvent
		if err = rows.Scan(&e.JobID, &e.Seq, &e.Kind, &e.Payload, &e.CreatedAt); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteBefore removes events older than cutoff and returns how many.
func (r *JobEventsRepository) DeleteBefore(
	ctx context.Context,
	cutoff time.Time,
) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM factory.job_events WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, postgres.PgxErrorToHTTPError(err)
	}
	return tag.RowsAffected(), nil
}
