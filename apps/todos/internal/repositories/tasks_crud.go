package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

type TasksRepository struct {
	db postgres.DB
}

func (r *TasksRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Task, error) {
	var t models.Task
	err := r.db.QueryRow(ctx, `
		SELECT id, owner_user_id, title, description, labels,
		       status, priority, sort_order, completed_at, archived_at, due_date,
		       deadline, created_at, updated_at, section_id, workspace_id,
		       recur_days, recur_rule
		FROM todos.tasks
		WHERE id = $1 AND owner_user_id = $2`,
		id, userID,
	).Scan(
		&t.ID, &t.OwnerUserID, &t.Title, &t.Description,
		&t.Labels, &t.Status,
		&t.Priority, &t.SortOrder,
		&t.CompletedAt, &t.ArchivedAt, &t.DueDate, &t.Deadline,
		&t.CreatedAt, &t.UpdatedAt, &t.SectionID, &t.WorkspaceID, &t.RecurDays,
		&t.RecurRule,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	links, err := r.getLinks(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Links = links
	subtasks, err := r.getSubtasks(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Subtasks = subtasks
	t.SubtaskTotal = len(subtasks)
	for _, s := range subtasks {
		if s.Done {
			t.SubtaskDone++
		}
	}
	return &t, nil
}

// maxSortOrderForEffPriority returns the highest sort_order among open tasks
// whose effective priority (0→999) is <= effPriority. Returns -1 if none exist.
func (r *TasksRepository) maxSortOrderForEffPriority(
	ctx context.Context,
	userID string,
	effPriority int,
) (int, error) {
	var result int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(sort_order), -1)
		FROM todos.tasks
		WHERE owner_user_id = $1 AND status = 'open'
		  AND CASE WHEN priority = 0 THEN 999 ELSE priority END <= $2`,
		userID, effPriority,
	).Scan(&result)
	return result, err
}

func (r *TasksRepository) shiftSortOrdersAfter(
	ctx context.Context,
	userID string,
	after int,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE todos.tasks
		SET sort_order = sort_order + 1
		WHERE owner_user_id = $1 AND status = 'open' AND sort_order > $2`,
		userID, after,
	)
	return err
}

func (r *TasksRepository) Create(
	ctx context.Context,
	t models.Task,
) (*models.Task, error) {
	effPriority := t.Priority
	if effPriority == 0 {
		effPriority = 999
	}
	insertAfter, err := r.maxSortOrderForEffPriority(ctx, t.OwnerUserID, effPriority)
	if err != nil {
		return nil, err
	}
	if err = r.shiftSortOrdersAfter(ctx, t.OwnerUserID, insertAfter); err != nil {
		return nil, err
	}
	t.SortOrder = insertAfter + 1

	err = r.db.QueryRow(ctx, `
		INSERT INTO todos.tasks
		    (owner_user_id, title, description, labels,
		     due_date, deadline, section_id, workspace_id, priority, sort_order,
		     recur_days, recur_rule)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at`,
		t.OwnerUserID, t.Title, t.Description,
		t.Labels, t.DueDate, t.Deadline,
		t.SectionID, t.WorkspaceID,
		t.Priority, t.SortOrder, t.RecurDays, t.RecurRule,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Status = models.StatusOpen
	return &t, nil
}

func (r *TasksRepository) Update(
	ctx context.Context,
	t models.Task,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE todos.tasks
		SET title = $1, description = $2, labels = $3,
		    due_date = $4, deadline = $5, section_id = $6,
		    priority = $7, recur_days = $8, recur_rule = $9,
		    updated_at = now()
		WHERE id = $10 AND owner_user_id = $11`,
		t.Title, t.Description, t.Labels,
		t.DueDate, t.Deadline, t.SectionID,
		t.Priority, t.RecurDays, t.RecurRule, t.ID, t.OwnerUserID,
	)
	return err
}

func (r *TasksRepository) MoveSection(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	sectionID *uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE todos.tasks
		SET section_id = $1, updated_at = now()
		WHERE id = $2 AND owner_user_id = $3`,
		sectionID, id, userID,
	)
	return err
}

func (r *TasksRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM todos.tasks WHERE id = $1 AND owner_user_id = $2`,
		id, userID,
	)
	return err
}

func (r *TasksRepository) SetStatus(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	status string,
	completedAt *time.Time,
	archivedAt *time.Time,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE todos.tasks
		SET status = $1, completed_at = $2, archived_at = $3, updated_at = now()
		WHERE id = $4 AND owner_user_id = $5`,
		status, completedAt, archivedAt, id, userID,
	)
	return err
}

// ReorderTasks sets sort_order = 0,1,2,... for each task ID in the given order.
// Only updates tasks owned by userID.
func (r *TasksRepository) ReorderTasks(
	ctx context.Context,
	userID string,
	ids []uuid.UUID,
) error {
	for i, id := range ids {
		_, err := r.db.Exec(ctx, `
			UPDATE todos.tasks SET sort_order = $1
			WHERE id = $2 AND owner_user_id = $3 AND status = 'open'`,
			i, id, userID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
