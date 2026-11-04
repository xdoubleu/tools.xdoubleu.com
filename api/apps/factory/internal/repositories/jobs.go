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
	finished_at, pr_url, runner_id, pr_state, pr_ci, pr_review`

func scanJob(row rowScanner) (*models.Job, error) {
	var j models.Job
	err := row.Scan(
		&j.ID, &j.TaskID, &j.RepoID, &j.Harness, &j.Prompt, &j.Status, &j.Branch,
		&j.SessionID, &j.Outcome, &j.CreatedBy, &j.CreatedAt, &j.StartedAt,
		&j.LastCheckIn, &j.FinishedAt, &j.PRURL,
		&j.RunnerID, &j.PRState, &j.PRCI, &j.PRReview,
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
func (r *JobsRepository) Claim(
	ctx context.Context,
	runnerID uuid.UUID,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET
			status = 'running', started_at = now(), last_check_in = now(),
			runner_id = $1
		WHERE id = (
			SELECT id FROM factory.jobs WHERE status = 'queued'
			ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		AND NOT EXISTS (SELECT 1 FROM factory.jobs WHERE status = 'running')
		RETURNING `+jobColumns, runnerID))
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

// Complete finishes a running job, for the runner that claimed it (or any runner,
// if the job was claimed before runners were recorded). A job that is no longer
// running (it was lost or cancelled meanwhile) or is another runner's is not found.
func (r *JobsRepository) Complete(
	ctx context.Context,
	id uuid.UUID,
	status models.JobStatus,
	outcome, sessionID, prURL string,
	runnerID uuid.UUID,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET
			status = $2, outcome = $3, finished_at = now(),
			session_id = COALESCE(NULLIF($4, ''), session_id),
			pr_url = $5
		WHERE id = $1 AND status = 'running'
			AND (runner_id IS NULL OR runner_id = $6)
		RETURNING `+jobColumns, id, status, outcome, sessionID, prURL, runnerID))
}

// CancelForTask ends the task's queued or running job (a task has at most one).
// A task with none is not found.
func (r *JobsRepository) CancelForTask(
	ctx context.Context,
	taskID uuid.UUID,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET status = 'cancelled', finished_at = now()
		WHERE task_id = $1 AND status IN ('queued', 'running')
		RETURNING `+jobColumns, taskID))
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

// Running returns which of the jobs are running.
func (r *JobsRepository) Running(
	ctx context.Context,
	ids []uuid.UUID,
) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id FROM factory.jobs WHERE id = ANY($1) AND status = 'running'`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// WithOpenPullRequest lists jobs whose pull request is not over, the ones read
// longest ago first, so a limit still gets to all of them in turn.
func (r *JobsRepository) WithOpenPullRequest(
	ctx context.Context,
	limit int,
) ([]models.Job, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+jobColumns+` FROM factory.jobs
		WHERE pr_url <> '' AND pr_state NOT IN ('merged', 'closed')
		ORDER BY pr_checked_at NULLS FIRST, created_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
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

// MarkPullRequestPolled notes that the poller tried the job, so one that keeps
// failing takes its turn at the back instead of holding the front of every batch.
func (r *JobsRepository) MarkPullRequestPolled(
	ctx context.Context,
	id uuid.UUID,
) error {
	_, err := r.db.Exec(ctx,
		`UPDATE factory.jobs SET pr_checked_at = now() WHERE id = $1`, id)
	return postgres.PgxErrorToHTTPError(err)
}

// SetPullRequestStatus records what the poller read.
func (r *JobsRepository) SetPullRequestStatus(
	ctx context.Context,
	id uuid.UUID,
	state, ci, review string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE factory.jobs
		SET pr_state = $2, pr_ci = $3, pr_review = $4, pr_checked_at = now()
		WHERE id = $1`, id, state, ci, review)
	return postgres.PgxErrorToHTTPError(err)
}
