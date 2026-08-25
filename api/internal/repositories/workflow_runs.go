package repositories

import (
	"context"
	"time"

	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/models"
)

// WorkflowRunsRepository stores GitHub Actions workflow run/job history
// (global.workflow_run_samples/global.workflow_job_samples), persisted by
// internal/observability/jobs.WorkflowRunsSnapshotJob so duration/failure
// trends survive past internal/github.Client's 45s in-memory cache
// (issue #1217).
type WorkflowRunsRepository struct {
	db postgres.DB
}

func NewWorkflowRunsRepository(db postgres.DB) *WorkflowRunsRepository {
	return &WorkflowRunsRepository{db: db}
}

// Exists reports whether runID has already been recorded.
func (r *WorkflowRunsRepository) Exists(
	ctx context.Context, runID int64,
) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM global.workflow_run_samples WHERE run_id = $1)",
		runID,
	).Scan(&exists)
	return exists, err
}

// InsertRun records one completed workflow run.
func (r *WorkflowRunsRepository) InsertRun(
	ctx context.Context, run models.WorkflowRunSample,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO global.workflow_run_samples (
			run_id, workflow_name, branch, event, conclusion, url,
			duration_ms, started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (run_id) DO NOTHING
	`,
		run.RunID, run.WorkflowName, run.Branch, run.Event, run.Conclusion,
		run.URL, run.DurationMs, run.StartedAt, run.CompletedAt,
	)
	return err
}

// InsertJobs records the per-job breakdown of one workflow run.
func (r *WorkflowRunsRepository) InsertJobs(
	ctx context.Context, jobs []models.WorkflowJobSample,
) error {
	for _, j := range jobs {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO global.workflow_job_samples (
				run_id, job_name, conclusion, duration_ms, started_at, completed_at
			) VALUES ($1, $2, $3, $4, $5, $6)
		`, j.RunID, j.JobName, j.Conclusion, j.DurationMs, j.StartedAt, j.CompletedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// MainFailures returns recorded runs on branch that ended with conclusion,
// most recent first — used to flag main-branch CI failures, which should
// never happen since main deploys straight off a passing push.
func (r *WorkflowRunsRepository) MainFailures(
	ctx context.Context, branch, conclusion string, since time.Time,
) ([]models.WorkflowRunSample, error) {
	rows, err := r.db.Query(ctx, `
		SELECT run_id, workflow_name, branch, event, conclusion, url,
			duration_ms, started_at, completed_at
		FROM global.workflow_run_samples
		WHERE branch = $1 AND conclusion = $2 AND started_at >= $3
		ORDER BY started_at DESC
	`, branch, conclusion, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []models.WorkflowRunSample
	for rows.Next() {
		var run models.WorkflowRunSample
		if err = rows.Scan(
			&run.RunID, &run.WorkflowName, &run.Branch, &run.Event,
			&run.Conclusion, &run.URL, &run.DurationMs, &run.StartedAt,
			&run.CompletedAt,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// WorkflowDurationStats aggregates completed runs' durations per workflow
// name since the given time, most-runs-first.
func (r *WorkflowRunsRepository) WorkflowDurationStats(
	ctx context.Context, since time.Time,
) ([]models.WorkflowDurationStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT workflow_name,
			avg(duration_ms) AS avg_ms,
			percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms) AS p95_ms,
			count(*) AS run_count
		FROM global.workflow_run_samples
		WHERE started_at >= $1 AND conclusion <> ''
		GROUP BY workflow_name
		ORDER BY run_count DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.WorkflowDurationStat
	for rows.Next() {
		var s models.WorkflowDurationStat
		if err = rows.Scan(
			&s.WorkflowName, &s.AvgDurationMs, &s.P95DurationMs, &s.RunCount,
		); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// JobDurationStats aggregates recorded jobs' durations per job name since
// the given time — the per-action breakdown.
func (r *WorkflowRunsRepository) JobDurationStats(
	ctx context.Context, since time.Time,
) ([]models.JobDurationStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT j.job_name,
			avg(j.duration_ms) AS avg_ms,
			percentile_cont(0.95) WITHIN GROUP (ORDER BY j.duration_ms) AS p95_ms,
			count(*) AS run_count
		FROM global.workflow_job_samples j
		WHERE j.started_at >= $1
		GROUP BY j.job_name
		ORDER BY run_count DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.JobDurationStat
	for rows.Next() {
		var s models.JobDurationStat
		if err = rows.Scan(
			&s.JobName, &s.AvgDurationMs, &s.P95DurationMs, &s.RunCount,
		); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// PruneOlderThan deletes run/job samples started before cutoff.
func (r *WorkflowRunsRepository) PruneOlderThan(
	ctx context.Context, cutoff time.Time,
) error {
	if _, err := r.db.Exec(ctx, `
		DELETE FROM global.workflow_job_samples
		WHERE run_id IN (
			SELECT run_id FROM global.workflow_run_samples WHERE started_at < $1
		)
	`, cutoff); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx,
		"DELETE FROM global.workflow_run_samples WHERE started_at < $1", cutoff,
	)
	return err
}
