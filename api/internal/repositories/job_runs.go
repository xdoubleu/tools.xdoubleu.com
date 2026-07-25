package repositories

import (
	"context"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/internal/models"
)

// jobRunsRetention bounds global.job_runs; older rows are pruned on insert
// so no separate cleanup job is needed.
const jobRunsRetention = 90 * 24 * time.Hour

type JobRunsRepository struct {
	db postgres.DB
}

func NewJobRunsRepository(db postgres.DB) *JobRunsRepository {
	return &JobRunsRepository{db: db}
}

func (r *JobRunsRepository) Insert(ctx context.Context, run models.JobRun) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO global.job_runs (job_id, started_at, duration_ms, success, error)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
	`, run.JobID, run.StartedAt, run.DurationMs, run.Success, run.Error)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx,
		`DELETE FROM global.job_runs WHERE started_at < $1`,
		time.Now().Add(-jobRunsRetention),
	)
	return err
}

// Stats aggregates runs per job since the given time.
func (r *JobRunsRepository) Stats(
	ctx context.Context,
	since time.Time,
) ([]models.JobStats, error) {
	rows, err := r.db.Query(ctx, `
		SELECT job_id,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE NOT success),
		       COALESCE(AVG(duration_ms), 0)::bigint,
		       MAX(started_at)
		FROM global.job_runs
		WHERE started_at >= $1
		GROUP BY job_id
		ORDER BY job_id
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.JobStats
	for rows.Next() {
		var s models.JobStats
		if err = rows.Scan(
			&s.JobID,
			&s.TotalRuns,
			&s.FailedRuns,
			&s.AvgDurationMs,
			&s.LastRunAt,
		); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// ListRecent returns the most recent runs across all jobs, newest first.
func (r *JobRunsRepository) ListRecent(
	ctx context.Context,
	since time.Time,
	limit int,
) ([]models.JobRun, error) {
	rows, err := r.db.Query(ctx, `
		SELECT job_id, started_at, duration_ms, success, COALESCE(error, '')
		FROM global.job_runs
		WHERE started_at >= $1
		ORDER BY started_at DESC
		LIMIT $2
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []models.JobRun
	for rows.Next() {
		var run models.JobRun
		if err = rows.Scan(
			&run.JobID,
			&run.StartedAt,
			&run.DurationMs,
			&run.Success,
			&run.Error,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	return runs, rows.Err()
}
