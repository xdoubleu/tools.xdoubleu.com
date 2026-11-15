package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

type JobsRepository struct {
	db postgres.DB
}

const jobColumns = `id,
	COALESCE(task_id, '00000000-0000-0000-0000-000000000000'), repo_id, harness,
	prompt, status, branch,
	session_id, outcome, created_by, created_at, started_at, last_check_in,
	finished_at, pr_url, runner_id, pr_state, pr_ci, pr_review, parent_job_id,
	model, limited, limit_resets_at,
	COALESCE((SELECT number FROM factory.tasks WHERE id = task_id), 0), routine`

func scanJob(row rowScanner) (*models.Job, error) {
	var j models.Job
	err := row.Scan(
		&j.ID, &j.TaskID, &j.RepoID, &j.Harness, &j.Prompt, &j.Status, &j.Branch,
		&j.SessionID, &j.Outcome, &j.CreatedBy, &j.CreatedAt, &j.StartedAt,
		&j.LastCheckIn, &j.FinishedAt, &j.PRURL,
		&j.RunnerID, &j.PRState, &j.PRCI, &j.PRReview, &j.ParentJobID,
		&j.Model, &j.Limited, &j.LimitResetsAt, &j.TaskNumber, &j.Routine,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &j, nil
}

// Create queues a job, which has no harness or model until a runner claims it.
func (r *JobsRepository) Create(
	ctx context.Context,
	j models.Job,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		INSERT INTO factory.jobs (
			task_id, repo_id, prompt, branch, created_by, parent_job_id, routine
		)
		VALUES (NULLIF($1, '00000000-0000-0000-0000-000000000000'::uuid),
			$2, $3, $4, $5, $6, $7)
		RETURNING `+jobColumns,
		j.TaskID, j.RepoID, j.Prompt, j.Branch, j.CreatedBy, j.ParentJobID, j.Routine))
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

// Claim starts the oldest queued job unless the runner already runs maxJobs;
// both are not found. The job takes the harness and model the runner runs on, and
// with skipClaude a Claude Code runner finds none. The runner's row is locked
// meanwhile, so its concurrent claims count one another, and SKIP LOCKED keeps
// any two from taking one job.
func (r *JobsRepository) Claim(
	ctx context.Context,
	runnerID uuid.UUID,
	maxJobs int,
	harness models.Harness,
	model string,
	skipClaude bool,
) (*models.Job, error) {
	if skipClaude && harness == models.HarnessClaudeCode {
		return nil, database.ErrResourceNotFound
	}
	// A held claim polls: a runner already at its cap is turned away before it
	// takes the row lock.
	var running int
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM factory.jobs
		WHERE status = 'running' AND runner_id = $1`, runnerID).Scan(&running); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if running >= maxJobs {
		return nil, database.ErrResourceNotFound
	}

	//nolint:exhaustruct // default tx options
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// A lock taken in its own statement, so the count that follows sees the claim
	// that held it.
	if _, err = tx.Exec(ctx,
		`SELECT 1 FROM factory.runners WHERE id = $1 FOR UPDATE`, runnerID,
	); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if err = tx.QueryRow(ctx, `
		SELECT count(*) FROM factory.jobs
		WHERE status = 'running' AND runner_id = $1`, runnerID).Scan(&running); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if running >= maxJobs {
		return nil, database.ErrResourceNotFound
	}
	job, err := scanJob(tx.QueryRow(ctx, `
		UPDATE factory.jobs SET
			status = 'running', started_at = now(), last_check_in = now(),
			runner_id = $1, harness = $2, model = $3
		WHERE id = (
			SELECT id FROM factory.jobs WHERE status = 'queued'
			ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		RETURNING `+jobColumns, runnerID, harness, model))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return job, nil
}

// OldestQueued returns the job that has waited longest for a runner; none queued
// is not found.
func (r *JobsRepository) OldestQueued(ctx context.Context) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		SELECT `+jobColumns+` FROM factory.jobs WHERE status = 'queued'
		ORDER BY created_at, id LIMIT 1`))
}

// AnyRunning reports whether any job is running.
func (r *JobsRepository) AnyRunning(ctx context.Context) (bool, error) {
	var running bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM factory.jobs WHERE status = 'running')`,
	).Scan(&running)
	return running, err
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
	limited bool,
	limitResetsAt *time.Time,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET
			status = $2, outcome = $3, finished_at = now(),
			session_id = COALESCE(NULLIF($4, ''), session_id),
			pr_url = $5, limited = $7, limit_resets_at = $8
		WHERE id = $1 AND status = 'running'
			AND (runner_id IS NULL OR runner_id = $6)
		RETURNING `+jobColumns, id, status, outcome, sessionID, prURL, runnerID,
		limited, limitResetsAt))
}

// ActiveLimit is the latest time a limited job's limit lifts, among those still in
// force, or nil when none is. A limit whose reset time the CLI did not say is held
// for an hour after the job ended, so the board does not pretend it has lifted.
func (r *JobsRepository) ActiveLimit(ctx context.Context) (*time.Time, error) {
	var until *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT max(until) FROM (
			SELECT COALESCE(limit_resets_at, finished_at + interval '1 hour') AS until
			FROM factory.jobs WHERE limited
		) limits
		WHERE until > now()`).Scan(&until)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return until, nil
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

// WithPullRequest returns which of the given tasks have a job that opened a pull
// request.
func (r *JobsRepository) WithPullRequest(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT task_id FROM factory.jobs
		WHERE task_id = ANY($1) AND pr_url <> ''`, ids)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
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

// LastForRoutine returns the routine's newest job; none is not found.
func (r *JobsRepository) LastForRoutine(
	ctx context.Context,
	routine string,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		SELECT `+jobColumns+` FROM factory.jobs WHERE routine = $1
		ORDER BY created_at DESC, id LIMIT 1`, routine))
}

// CancelRoutine ends the routine's queued or running job (it has at most one).
// A routine with none is not found.
func (r *JobsRepository) CancelRoutine(
	ctx context.Context,
	routine string,
) (*models.Job, error) {
	return scanJob(r.db.QueryRow(ctx, `
		UPDATE factory.jobs SET status = 'cancelled', finished_at = now()
		WHERE routine = $1 AND status IN ('queued', 'running')
		RETURNING `+jobColumns, routine))
}

// RecordTimings stores a finished job's phase times, for the runner that ran it,
// once (a repeated report is not found).
func (r *JobsRepository) RecordTimings(
	ctx context.Context,
	id, runnerID uuid.UUID,
	t models.JobTimings,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE factory.jobs SET
			agent_seconds = $3, publish_seconds = $4, ci_seconds = $5,
			merge_seconds = $6, total_seconds = $7, gate_retries = $8,
			push_retries = $9, ci_retries = $10
		WHERE id = $1 AND status NOT IN ('queued', 'running')
			AND runner_id = $2 AND total_seconds IS NULL`,
		id, runnerID, t.Agent, t.Publish, t.CI, t.Merge, t.Total,
		t.GateRetries, t.PushRetries, t.CIRetries)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}
