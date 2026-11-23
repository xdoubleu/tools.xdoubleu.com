package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"tools.xdoubleu.com/internal/kit/database"
	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/platform/factory/internal/models"
)

type ChatsRepository struct {
	db postgres.DB
}

const chatColumns = `id, repo_id,
	COALESCE(task_id, '00000000-0000-0000-0000-000000000000'), harness, runner_id,
	COALESCE(job_id, '00000000-0000-0000-0000-000000000000'), state, branch, report,
	last_seq, created_by, created_at, updated_at, stopped_at`

func scanChat(row rowScanner) (*models.ChatSession, error) {
	var c models.ChatSession
	err := row.Scan(
		&c.ID, &c.RepoID, &c.TaskID, &c.Harness, &c.RunnerID, &c.JobID, &c.State,
		&c.Branch, &c.Report, &c.LastSeq, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
		&c.StoppedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &c, nil
}

// Start pins a new session to its runner and creates its running chat job, unless
// the runner already runs maxJobs jobs, which is a conflict. The runner's row is
// locked meanwhile, so concurrent starts and claims count one another.
func (r *ChatsRepository) Start(
	ctx context.Context,
	c models.ChatSession,
	createdBy string,
) (*models.ChatSession, error) {
	//nolint:exhaustruct // default tx options
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var maxJobs, running int
	if err = tx.QueryRow(ctx,
		`SELECT max_jobs FROM factory.runners WHERE id = $1 FOR UPDATE`, c.RunnerID,
	).Scan(&maxJobs); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if err = tx.QueryRow(ctx, `
		SELECT count(*) FROM factory.jobs
		WHERE status = 'running' AND runner_id = $1`, c.RunnerID).Scan(&running); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if running >= maxJobs {
		return nil, database.ErrResourceConflict
	}

	var jobID uuid.UUID
	if err = tx.QueryRow(ctx, `
		INSERT INTO factory.jobs (
			kind, repo_id, harness, prompt, branch, status, created_by, runner_id,
			started_at, last_check_in
		)
		VALUES ('chat', $1, $2, '', $3, 'running', $4, $5, now(), now())
		RETURNING id`,
		c.RepoID, c.Harness, c.Branch, createdBy, c.RunnerID,
	).Scan(&jobID); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	session, err := scanChat(tx.QueryRow(ctx, `
		INSERT INTO factory.chat_sessions (
			repo_id, task_id, harness, runner_id, job_id, branch, created_by
		)
		VALUES ($1, NULLIF($2, '00000000-0000-0000-0000-000000000000'::uuid),
			$3, $4, $5, $6, $7)
		RETURNING `+chatColumns,
		c.RepoID, c.TaskID, c.Harness, c.RunnerID, jobID, c.Branch, createdBy))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return session, nil
}

// Get returns one session.
func (r *ChatsRepository) Get(
	ctx context.Context,
	id uuid.UUID,
) (*models.ChatSession, error) {
	return scanChat(r.db.QueryRow(ctx,
		`SELECT `+chatColumns+` FROM factory.chat_sessions WHERE id = $1`, id))
}

// ForJob returns the session of a chat job.
func (r *ChatsRepository) ForJob(
	ctx context.Context,
	jobID uuid.UUID,
) (*models.ChatSession, error) {
	return scanChat(r.db.QueryRow(ctx,
		`SELECT `+chatColumns+` FROM factory.chat_sessions WHERE job_id = $1`, jobID))
}

// Ask appends a question event and moves the session to asking in one
// statement; a stopped or unknown session is a conflict.
func (r *ChatsRepository) Ask(
	ctx context.Context,
	id uuid.UUID,
	payload string,
) (int, error) {
	var seq int
	err := r.db.QueryRow(ctx, `
		WITH s AS (
			UPDATE factory.chat_sessions
			SET last_seq = last_seq + 1, state = 'asking', updated_at = now()
			WHERE id = $1 AND state <> 'stopped'
			RETURNING last_seq
		), e AS (
			INSERT INTO factory.chat_events (session_id, seq, kind, payload)
			SELECT $1, s.last_seq, $3, $2 FROM s
		)
		SELECT last_seq FROM s`, id, payload, models.ChatEventAsk).Scan(&seq)
	if err != nil {
		return 0, notFoundIsConflict(postgres.PgxErrorToHTTPError(err))
	}
	return seq, nil
}

// List returns the newest sessions first, without the stopped ones when
// activeOnly.
func (r *ChatsRepository) List(
	ctx context.Context,
	activeOnly bool,
	limit int,
) ([]models.ChatSession, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+chatColumns+` FROM factory.chat_sessions
		WHERE NOT $1 OR state <> 'stopped'
		ORDER BY created_at DESC, id LIMIT $2`, activeOnly, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := []models.ChatSession{}
	for rows.Next() {
		c, scanErr := scanChat(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// Transition moves a session from one of the from states to the new state; a
// session in none of them is a conflict.
func (r *ChatsRepository) Transition(
	ctx context.Context,
	id uuid.UUID,
	from []models.ChatState,
	to models.ChatState,
) (*models.ChatSession, error) {
	states := make([]string, len(from))
	for i, f := range from {
		states[i] = string(f)
	}
	c, err := scanChat(r.db.QueryRow(ctx, `
		UPDATE factory.chat_sessions SET state = $3, updated_at = now()
		WHERE id = $1 AND state = ANY($2::text[])
		RETURNING `+chatColumns, id, states, to))
	if err != nil {
		return nil, notFoundIsConflict(err)
	}
	return c, nil
}

// notFoundIsConflict reads a guarded update that matched no row as the guard failing.
func notFoundIsConflict(err error) error {
	if errors.Is(err, database.ErrResourceNotFound) {
		return database.ErrResourceConflict
	}
	return err
}

// Append adds events to a running session, numbered after its newest; a stopped
// or unknown session is a conflict. The events of one call keep their order.
func (r *ChatsRepository) Append(
	ctx context.Context,
	id uuid.UUID,
	events []models.ChatEventInput,
) error {
	return appendChatEvents(ctx, r.db, id, events, false)
}

// chatExec is what appending events needs of a database or a transaction.
type chatExec interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func appendChatEvents(
	ctx context.Context,
	db chatExec,
	id uuid.UUID,
	events []models.ChatEventInput,
	toStopped bool,
) error {
	kinds := make([]string, len(events))
	payloads := make([]string, len(events))
	for i, e := range events {
		kinds[i], payloads[i] = e.Kind, e.Payload
	}
	tag, err := db.Exec(ctx, `
		WITH s AS (
			UPDATE factory.chat_sessions
			SET last_seq = last_seq + $2::int, updated_at = now()
			WHERE id = $1 AND (state <> 'stopped' OR $5)
			RETURNING last_seq
		)
		INSERT INTO factory.chat_events (session_id, seq, kind, payload)
		SELECT $1, s.last_seq - $2::int + t.n::int, t.kind, t.payload
		FROM s, unnest($3::text[], $4::text[]) WITH ORDINALITY AS t(kind, payload, n)`,
		id, len(events), kinds, payloads, toStopped)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceConflict
	}
	return nil
}

// After lists a session's events with seq above afterSeq, oldest first.
func (r *ChatsRepository) After(
	ctx context.Context,
	id uuid.UUID,
	afterSeq, limit int,
) ([]models.ChatEvent, error) {
	rows, err := r.db.Query(ctx, `
		SELECT session_id, seq, kind, payload, created_at
		FROM factory.chat_events
		WHERE session_id = $1 AND seq > $2
		ORDER BY seq LIMIT $3`, id, afterSeq, limit)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	out := []models.ChatEvent{}
	for rows.Next() {
		var e models.ChatEvent
		if err = rows.Scan(
			&e.SessionID,
			&e.Seq,
			&e.Kind,
			&e.Payload,
			&e.CreatedAt,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Stop ends a session that is still going: it keeps the report, appends the
// stopped event and finishes the chat job, which frees the runner's slot. A
// session that already stopped is a conflict.
func (r *ChatsRepository) Stop(
	ctx context.Context,
	id uuid.UUID,
	report string,
	jobStatus models.JobStatus,
) (*models.ChatSession, error) {
	//nolint:exhaustruct // default tx options
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	c, err := scanChat(tx.QueryRow(ctx, `
		UPDATE factory.chat_sessions
		SET state = 'stopped', report = $2, stopped_at = now(), updated_at = now()
		WHERE id = $1 AND state <> 'stopped'
		RETURNING `+chatColumns, id, report))
	if err != nil {
		return nil, notFoundIsConflict(err)
	}
	if err = appendChatEvents(ctx, tx, id, []models.ChatEventInput{
		{Kind: models.ChatEventStopped, Payload: report},
	}, true); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `
		UPDATE factory.jobs SET status = $2, finished_at = now(), outcome = $3
		WHERE id = $1 AND status = 'running'`, c.JobID, jobStatus, report); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	c.LastSeq++
	return c, nil
}

// StopForJob stops the unstopped session of a chat job that ended without it,
// keeping the reason as its report; none is not found.
func (r *ChatsRepository) StopForJob(
	ctx context.Context,
	jobID uuid.UUID,
	report string,
) (*models.ChatSession, error) {
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, `
		SELECT id FROM factory.chat_sessions
		WHERE job_id = $1 AND state <> 'stopped'`, jobID).Scan(&id); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return r.Stop(ctx, id, report, models.JobLost)
}

// ClaimFor marks the oldest unclaimed, unstopped session pinned to the runner as
// claimed and returns it; none is not found.
func (r *ChatsRepository) ClaimFor(
	ctx context.Context,
	runnerID uuid.UUID,
) (*models.ChatSession, error) {
	return scanChat(r.db.QueryRow(ctx, `
		UPDATE factory.chat_sessions SET claimed_at = now()
		WHERE id = (
			SELECT id FROM factory.chat_sessions
			WHERE runner_id = $1 AND claimed_at IS NULL AND state <> 'stopped'
			ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		RETURNING `+chatColumns, runnerID))
}

// AppendReport adds text to a session's report, after what a person's stop set.
func (r *ChatsRepository) AppendReport(
	ctx context.Context,
	id uuid.UUID,
	text string,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE factory.chat_sessions
		SET report = CASE WHEN report = '' THEN $2 ELSE report || E'\n\n' || $2 END,
			updated_at = now()
		WHERE id = $1`, id, text)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}
