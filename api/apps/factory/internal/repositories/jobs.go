package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type JobsRepository struct {
	db postgres.DB
}

const jobColumns = `id, task_id, repo_id, harness, prompt, status, branch,
	session_id, outcome, created_by, created_at, started_at, last_check_in,
	finished_at`

func scanJob(row rowScanner) (*models.Job, error) {
	var j models.Job
	err := row.Scan(
		&j.ID, &j.TaskID, &j.RepoID, &j.Harness, &j.Prompt, &j.Status, &j.Branch,
		&j.SessionID, &j.Outcome, &j.CreatedBy, &j.CreatedAt, &j.StartedAt,
		&j.LastCheckIn, &j.FinishedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &j, nil
}

// Create queues a job.
func (r *JobsRepository) Create(
	ctx context.Context,
	j models.Job,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		INSERT INTO factory.jobs (task_id, repo_id, harness, prompt, branch, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+jobColumns,
		j.TaskID, j.RepoID, j.Harness, j.Prompt, j.Branch, j.CreatedBy))
}

// Get returns one job.
func (r *JobsRepository) Get(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx,
		`SELECT `+jobColumns+` FROM factory.jobs WHERE id = $1`, id))
}

// ByTask returns a task's jobs, newest first.
func (r *JobsRepository) ByTask(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.Job, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+jobColumns+` FROM factory.jobs
		WHERE task_id = $1 ORDER BY created_at DESC, id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Job{}
	for rows.Next() {
		j, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// Claim starts the oldest queued job. Nothing queued, or a job already running
// (the one-running index), is not found.
func (r *JobsRepository) Claim(ctx context.Context) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET
			status = 'running', started_at = now(), last_check_in = now()
		WHERE id = (
			SELECT id FROM factory.jobs WHERE status = 'queued'
			ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		AND NOT EXISTS (SELECT 1 FROM factory.jobs WHERE status = 'running')
		RETURNING `+jobColumns))
}

// CheckIn records that a running job is alive and keeps its session id. A job
// that isn't running (lost, cancelled) is not found, which tells the runner to
// stop.
func (r *JobsRepository) CheckIn(
	ctx context.Context,
	id uuid.UUID,
	sessionID string,
) error {
	_, err := scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET
			last_check_in = now(),
			session_id = COALESCE(NULLIF($2, ''), session_id)
		WHERE id = $1 AND status = 'running'
		RETURNING `+jobColumns, id, sessionID))
	return err
}

// Complete finishes a running job. A job that is no longer running (it was
// lost or cancelled meanwhile) is not found.
func (r *JobsRepository) Complete(
	ctx context.Context,
	id uuid.UUID,
	status models.JobStatus,
	outcome, sessionID string,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET
			status = $2, outcome = $3, finished_at = now(),
			session_id = COALESCE(NULLIF($4, ''), session_id)
		WHERE id = $1 AND status = 'running'
		RETURNING `+jobColumns, id, status, outcome, sessionID))
}

// ExpireLost marks every running job that hasn't checked in since cutoff as
// lost and returns them.
func (r *JobsRepository) ExpireLost(
	ctx context.Context,
	cutoff time.Time,
) ([]models.Job, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE factory.jobs SET status = 'lost', finished_at = now()
		WHERE status = 'running' AND last_check_in < $1
		RETURNING `+jobColumns, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Job{}
	for rows.Next() {
		j, scanErr := scanJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}
