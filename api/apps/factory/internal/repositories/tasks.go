package repositories

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/factory/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
)

type TasksRepository struct {
	db postgres.DB
}

type rowScanner interface {
	Scan(dest ...any) error
}

// listColumns leaves out body: list rows never carry the wide column.
const listColumns = `id, repo_id, title, '', status, priority, labels, created_by,
	created_at, updated_at,
	(SELECT count(*) FROM factory.task_comments c WHERE c.task_id = t.id)`

const getColumns = `id, repo_id, title, body, status, priority, labels, created_by,
	created_at, updated_at,
	(SELECT count(*) FROM factory.task_comments c WHERE c.task_id = t.id)`

func scanTask(row rowScanner) (*models.Task, error) {
	var t models.Task
	err := row.Scan(
		&t.ID, &t.RepoID, &t.Title, &t.Body, &t.Status, &t.Priority, &t.Labels,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &t.CommentCount,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &t, nil
}

// List returns every task without its body, most recently updated first.
func (r *TasksRepository) List(ctx context.Context) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+listColumns+`
		FROM factory.tasks t
		ORDER BY updated_at DESC, id`)
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
			UPDATE factory.tasks SET status = $2, updated_at = now()
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
