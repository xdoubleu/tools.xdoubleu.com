package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

// prFields is how many values prColumn holds.
const prFields = 4

type TasksRepository struct {
	db postgres.DB
}

type rowScanner interface {
	Scan(dest ...any) error
}

// prColumn is the pull request of the task's newest job that opened one, as
// {url, state, ci, review}; null when no job did.
const prColumn = `(SELECT ARRAY[j.pr_url, j.pr_state, j.pr_ci, j.pr_review]
		FROM factory.jobs j WHERE j.task_id = t.id AND j.pr_url <> ''
		ORDER BY j.created_at DESC, j.id LIMIT 1)`

// listColumns leaves out body: list rows never carry the wide column.
const listColumns = `id, repo_id, title, '', status, priority, labels, created_by,
	created_at, updated_at,
	(SELECT count(*) FROM factory.task_comments c WHERE c.task_id = t.id),
	` + prColumn + `,
	ready_body IS NOT NULL`

const getColumns = `id, repo_id, title, body, status, priority, labels, created_by,
	created_at, updated_at,
	(SELECT count(*) FROM factory.task_comments c WHERE c.task_id = t.id),
	` + prColumn + `,
	ready_body IS NOT NULL`

func scanTask(row rowScanner) (*models.Task, error) {
	var t models.Task
	var pr []string
	err := row.Scan(
		&t.ID, &t.RepoID, &t.Title, &t.Body, &t.Status, &t.Priority, &t.Labels,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CommentCount, &pr, &t.PickupApproved,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if len(pr) == prFields {
		t.PRURL, t.PRState, t.PRCI, t.PRReview = pr[0], pr[1], pr[2], pr[3]
	}
	return &t, nil
}

// List returns tasks without their bodies, newest first. A non-empty query keeps
// only tasks whose title or body contains it, ignoring case: the match runs in
// SQL so bodies needn't be loaded.
func (r *TasksRepository) List(
	ctx context.Context,
	query string,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+listColumns+`
		FROM factory.tasks t
		WHERE $1 = '' OR position(lower($1) in lower(t.title || E'\n' || t.body)) > 0
		ORDER BY updated_at DESC, id`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Task{}
	for rows.Next() {
		t, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Statuses returns the status of each of the given tasks that exists.
func (r *TasksRepository) Statuses(
	ctx context.Context,
	ids []uuid.UUID,
) (map[uuid.UUID]models.Status, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, status FROM factory.tasks WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[uuid.UUID]models.Status, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var status models.Status
		if err = rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		out[id] = status
	}
	return out, rows.Err()
}

func (r *TasksRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		SELECT `+getColumns+` FROM factory.tasks t WHERE id = $1`, id))
}

// Create inserts a task in Backlog.
func (r *TasksRepository) Create(
	ctx context.Context,
	t models.Task,
	actor models.Actor,
) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		WITH t AS (
			INSERT INTO factory.tasks
				(repo_id, title, body, priority, labels, created_by,
				 body_edited_by, body_edited_channel)
			VALUES ($1, $2, $3, $4, $5, $6, $6, $7)
			RETURNING *
		)
		SELECT `+getColumns+` FROM t`,
		t.RepoID, t.Title, t.Body, t.Priority, t.Labels, actor.String(),
		actor.Channel(),
	))
}

// Update replaces title, body, priority and labels. The body's editor is
// recorded only when the body changes.
func (r *TasksRepository) Update(
	ctx context.Context,
	t models.Task,
	actor models.Actor,
) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		WITH t AS (
			UPDATE factory.tasks SET
				title = $2,
				priority = $4,
				labels = $5,
				body_edited_by = CASE WHEN body IS DISTINCT FROM $3
					THEN $6 ELSE body_edited_by END,
				body_edited_channel = CASE WHEN body IS DISTINCT FROM $3
					THEN $7 ELSE body_edited_channel END,
				ready_title = CASE WHEN status = 'ready' AND (
					CASE WHEN body IS DISTINCT FROM $3 THEN $7
						ELSE body_edited_channel END
				) = 'browser' THEN $2 ELSE ready_title END,
				ready_body = CASE WHEN status = 'ready' AND (
					CASE WHEN body IS DISTINCT FROM $3 THEN $7
						ELSE body_edited_channel END
				) = 'browser' THEN $3 ELSE ready_body END,
				body = $3,
				updated_at = now()
			WHERE id = $1
			RETURNING *
		)
		SELECT `+getColumns+` FROM t`,
		t.ID, t.Title, t.Body, t.Priority, t.Labels, actor.String(),
		actor.Channel(),
	))
}

func (r *TasksRepository) Move(
	ctx context.Context,
	id uuid.UUID,
	status models.Status,
) (*models.Task, error) {
	return scanTask(r.db.QueryRow(ctx, `
		WITH t AS (
			UPDATE factory.tasks SET status = $2, updated_at = now(),
				ready_title = CASE WHEN $2 = 'ready' AND body_edited_channel = 'browser'
					THEN title END,
				ready_body = CASE WHEN $2 = 'ready' AND body_edited_channel = 'browser'
					THEN body END
			WHERE id = $1
			RETURNING *
		)
		SELECT `+getColumns+` FROM t`, id, status))
}

// AddComment appends a comment and bumps the task's updated_at.
func (r *TasksRepository) AddComment(
	ctx context.Context,
	taskID uuid.UUID,
	author string,
	body string,
) (*models.Comment, error) {
	var c models.Comment
	err := r.db.QueryRow(ctx, `
		WITH touched AS (
			UPDATE factory.tasks SET updated_at = now() WHERE id = $1
			RETURNING id
		)
		INSERT INTO factory.task_comments (task_id, author, body)
		SELECT id, $2, $3 FROM touched
		RETURNING id, task_id, author, body, created_at`,
		taskID, author, body,
	).Scan(&c.ID, &c.TaskID, &c.Author, &c.Body, &c.CreatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &c, nil
}

func (r *TasksRepository) Comments(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.Comment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, author, body, created_at
		FROM factory.task_comments
		WHERE task_id = $1
		ORDER BY created_at, id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Comment{}
	for rows.Next() {
		var c models.Comment
		if err = rows.Scan(
			&c.ID, &c.TaskID, &c.Author, &c.Body, &c.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// BodyEditedChannel is how the task's body was last edited ("browser" or not).
func (r *TasksRepository) BodyEditedChannel(
	ctx context.Context,
	id uuid.UUID,
) (string, error) {
	var channel string
	err := r.db.QueryRow(ctx,
		`SELECT body_edited_channel FROM factory.tasks WHERE id = $1`, id,
	).Scan(&channel)
	if err != nil {
		return "", postgres.PgxErrorToHTTPError(err)
	}
	return channel, nil
}

// PickupCandidate is a Ready task as a human approved it.
type PickupCandidate struct {
	TaskID uuid.UUID
	RepoID uuid.UUID
	Title  string
	Body   string
}

// NextReady is the oldest highest-priority Ready task of a repo that has auto pickup
// on, with its approved text. It finds none while the board is paused or any job is
// queued or running, so it is the only query a quiet claim needs. A task whose body
// was last edited outside a browser when it went Ready has no approved text and is
// skipped. None is not found.
func (r *TasksRepository) NextReady(ctx context.Context) (*PickupCandidate, error) {
	var c PickupCandidate
	err := r.db.QueryRow(ctx, `
		SELECT t.id, t.repo_id, t.ready_title, t.ready_body
		FROM factory.tasks t
		JOIN factory.repos r ON r.id = t.repo_id
		WHERE t.status = 'ready' AND r.auto_pickup AND t.ready_body IS NOT NULL
			AND NOT COALESCE(
				(SELECT pickup_paused FROM factory.board_settings), false
			)
			AND NOT EXISTS (
				SELECT 1 FROM factory.jobs j WHERE j.status IN ('queued', 'running')
			)
		ORDER BY t.priority, t.created_at, t.id
		LIMIT 1`).Scan(&c.TaskID, &c.RepoID, &c.Title, &c.Body)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &c, nil
}
