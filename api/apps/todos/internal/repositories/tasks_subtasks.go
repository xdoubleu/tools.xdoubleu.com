package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

func (r *TasksRepository) AddSubtask(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	title string,
	description string,
	priority int,
	labels []string,
	dueDate *time.Time,
	deadline *time.Time,
	parentSubtaskID *uuid.UUID,
) (*models.Subtask, error) {
	var s models.Subtask
	s.TaskID = taskID
	s.ParentSubtaskID = parentSubtaskID
	err := r.db.QueryRow(ctx, `
		INSERT INTO todos.subtasks
		    (task_id, title, description, priority, labels,
		     due_date, deadline, parent_subtask_id, sort_order)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8,
		    COALESCE((SELECT MAX(sort_order)+1 FROM todos.subtasks
		              WHERE task_id = $1 AND parent_subtask_id IS NOT DISTINCT FROM $8), 0)
		WHERE EXISTS (
		    SELECT 1 FROM todos.tasks
		    WHERE id = $1 AND owner_user_id = $9
		)
		RETURNING id, title, description, done, sort_order, priority, labels,
		          due_date, deadline, created_at, updated_at, parent_subtask_id`,
		taskID, title, description, priority, labels, dueDate, deadline,
		parentSubtaskID, userID,
	).
		Scan(
			&s.ID, &s.Title, &s.Description, &s.Done, &s.SortOrder, &s.Priority,
			&s.Labels, &s.DueDate, &s.Deadline, &s.CreatedAt, &s.UpdatedAt,
			&s.ParentSubtaskID,
		)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *TasksRepository) UpdateSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
	title string,
	description string,
	priority int,
	labels []string,
	dueDate *time.Time,
	deadline *time.Time,
) (*models.Subtask, error) {
	var s models.Subtask
	err := r.db.QueryRow(ctx, `
		UPDATE todos.subtasks
		SET title = $1, description = $2, priority = $3,
		    labels = $4, due_date = $5, deadline = $6
		WHERE id = $7 AND task_id = $8
		  AND EXISTS (
		      SELECT 1 FROM todos.tasks
		      WHERE id = $8 AND owner_user_id = $9
		  )
		RETURNING id, task_id, title, description, done, sort_order,
		          priority, labels, due_date, deadline, created_at`,
		title, description, priority, labels, dueDate, deadline,
		id, taskID, userID,
	).Scan(
		&s.ID, &s.TaskID, &s.Title, &s.Description, &s.Done, &s.SortOrder,
		&s.Priority, &s.Labels, &s.DueDate, &s.Deadline, &s.CreatedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &s, nil
}

func (r *TasksRepository) ToggleSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE todos.subtasks SET done = NOT done
		WHERE id = $1 AND task_id = $2
		  AND EXISTS (
		      SELECT 1 FROM todos.tasks
		      WHERE id = $2 AND owner_user_id = $3
		  )`,
		id, taskID, userID,
	)
	return err
}

func (r *TasksRepository) DeleteSubtask(
	ctx context.Context,
	id uuid.UUID,
	taskID uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM todos.subtasks
		WHERE id = $1 AND task_id = $2
		  AND EXISTS (
		      SELECT 1 FROM todos.tasks
		      WHERE id = $2 AND owner_user_id = $3
		  )`,
		id, taskID, userID,
	)
	return err
}

// GetSubtaskDepth returns the depth of a subtask in its hierarchy.
// Returns 0 for top-level subtasks, 1 for children of top-level, etc.
func (r *TasksRepository) GetSubtaskDepth(
	ctx context.Context,
	taskID uuid.UUID,
	subtaskID uuid.UUID,
) (int, error) {
	var depth int
	err := r.db.QueryRow(ctx, `
		WITH RECURSIVE depth_calc AS (
		  SELECT id, parent_subtask_id, 0 AS depth
		  FROM todos.subtasks
		  WHERE id = $1 AND task_id = $2
		  UNION ALL
		  SELECT s.id, s.parent_subtask_id, d.depth + 1
		  FROM todos.subtasks s
		  JOIN depth_calc d ON d.parent_subtask_id = s.id
		  WHERE s.task_id = $2
		)
		SELECT COALESCE(MAX(depth), 0) FROM depth_calc`,
		subtaskID, taskID,
	).Scan(&depth)
	if err != nil {
		return 0, err
	}
	return depth, nil
}

func (r *TasksRepository) ReorderSubtasks(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	ids []uuid.UUID,
	parentSubtaskID *uuid.UUID,
) error {
	for i, id := range ids {
		_, err := r.db.Exec(ctx, `
			UPDATE todos.subtasks SET sort_order = $1
			WHERE id = $2 AND task_id = $3
			  AND parent_subtask_id IS NOT DISTINCT FROM $5
			  AND EXISTS (
			      SELECT 1 FROM todos.tasks
			      WHERE id = $3 AND owner_user_id = $4
			  )`,
			i, id, taskID, userID, parentSubtaskID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *TasksRepository) ListSubtasksForTasks(
	ctx context.Context,
	taskIDs []uuid.UUID,
) (map[uuid.UUID][]models.Subtask, error) {
	strIDs := make([]string, len(taskIDs))
	for i, id := range taskIDs {
		strIDs[i] = id.String()
	}
	rows, err := r.db.Query(ctx, `
		WITH RECURSIVE sub_tree AS (
		  SELECT id, task_id, parent_subtask_id, title, description, done,
		         sort_order, priority, labels, due_date, deadline, created_at,
		         updated_at
		  FROM todos.subtasks
		  WHERE task_id = ANY($1::uuid[]) AND parent_subtask_id IS NULL
		  UNION ALL
		  SELECT s.id, s.task_id, s.parent_subtask_id, s.title, s.description,
		         s.done, s.sort_order, s.priority, s.labels, s.due_date,
		         s.deadline, s.created_at, s.updated_at
		  FROM todos.subtasks s
		  JOIN sub_tree st ON s.parent_subtask_id = st.id
		)
		SELECT id, task_id, parent_subtask_id, title, description, done,
		       sort_order, priority, labels, due_date, deadline, created_at,
		       updated_at
		FROM sub_tree
		ORDER BY task_id, sort_order, created_at`,
		strIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]models.Subtask)
	for rows.Next() {
		var s models.Subtask
		if err = rows.Scan(
			&s.ID, &s.TaskID, &s.ParentSubtaskID, &s.Title, &s.Description,
			&s.Done, &s.SortOrder, &s.Priority, &s.Labels, &s.DueDate,
			&s.Deadline, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result[s.TaskID] = append(result[s.TaskID], s)
	}
	return result, rows.Err()
}
