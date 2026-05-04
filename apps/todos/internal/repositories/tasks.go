package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/todos/internal/models"
)

type TasksRepository struct {
	db postgres.DB
}

func (r *TasksRepository) ListOpen(
	ctx context.Context,
	userID string,
	sectionID *uuid.UUID,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.setup_label, t.type_label, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days,
		       COUNT(s.id) FILTER (WHERE s.done) AS subtask_done,
		       COUNT(s.id)                        AS subtask_total
		FROM todos.tasks t
		LEFT JOIN todos.subtasks s ON s.task_id = t.id
		WHERE t.owner_user_id = $1 AND t.status = 'open'
		  AND (t.workspace_id = $2 OR ($2::uuid IS NULL AND t.workspace_id IS NULL))
		  AND (t.section_id  = $3 OR ($3::uuid IS NULL AND t.section_id  IS NULL))
		GROUP BY t.id
		ORDER BY t.sort_order ASC, t.created_at ASC`,
		userID, workspaceID, sectionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *TasksRepository) ListByStatus(
	ctx context.Context,
	userID string,
	status string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.setup_label, t.type_label, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days,
		       COUNT(s.id) FILTER (WHERE s.done)  AS subtask_done,
		       COUNT(s.id)                         AS subtask_total
		FROM todos.tasks t
		LEFT JOIN todos.subtasks s ON s.task_id = t.id
		WHERE t.owner_user_id = $1 AND t.status = $2
		  AND (t.workspace_id = $3 OR ($3::uuid IS NULL AND t.workspace_id IS NULL))
		GROUP BY t.id
		ORDER BY t.created_at DESC`,
		userID, status, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *TasksRepository) ListArchived(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.setup_label, t.type_label, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days,
		       COUNT(s.id) FILTER (WHERE s.done)  AS subtask_done,
		       COUNT(s.id)                         AS subtask_total
		FROM todos.tasks t
		LEFT JOIN todos.subtasks s ON s.task_id = t.id
		WHERE t.owner_user_id = $1 AND t.status = 'archived'
		  AND (t.workspace_id = $2 OR ($2::uuid IS NULL AND t.workspace_id IS NULL))
		  AND ($3 = '' OR t.title ILIKE '%' || $3 || '%'
		       OR t.description ILIKE '%' || $3 || '%')
		GROUP BY t.id
		ORDER BY t.archived_at DESC`,
		userID, workspaceID, query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *TasksRepository) SearchAll(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.setup_label, t.type_label, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days,
		       COUNT(s.id) FILTER (WHERE s.done)  AS subtask_done,
		       COUNT(s.id)                         AS subtask_total
		FROM todos.tasks t
		LEFT JOIN todos.subtasks s ON s.task_id = t.id
		WHERE t.owner_user_id = $1
		  AND (t.workspace_id = $2 OR ($2::uuid IS NULL AND t.workspace_id IS NULL))
		  AND (t.title ILIKE $3 OR t.description ILIKE $3)
		GROUP BY t.id
		ORDER BY t.status ASC, t.created_at DESC`,
		userID, workspaceID, "%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *TasksRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Task, error) {
	var t models.Task
	err := r.db.QueryRow(ctx, `
		SELECT id, owner_user_id, title, description, setup_label, type_label,
		       status, priority, sort_order, completed_at, archived_at, due_date,
		       created_at, updated_at, section_id, workspace_id, recur_days
		FROM todos.tasks
		WHERE id = $1 AND owner_user_id = $2`,
		id, userID,
	).Scan(
		&t.ID, &t.OwnerUserID, &t.Title, &t.Description,
		&t.SetupLabel, &t.TypeLabel, &t.Status,
		&t.Priority, &t.SortOrder,
		&t.CompletedAt, &t.ArchivedAt, &t.DueDate,
		&t.CreatedAt, &t.UpdatedAt, &t.SectionID, &t.WorkspaceID, &t.RecurDays,
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
		    (owner_user_id, title, description, setup_label, type_label,
		     due_date, section_id, workspace_id, priority, sort_order,
		     recur_days)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`,
		t.OwnerUserID, t.Title, t.Description,
		t.SetupLabel, t.TypeLabel, t.DueDate, t.SectionID, t.WorkspaceID,
		t.Priority, t.SortOrder, t.RecurDays,
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
		SET title = $1, description = $2, setup_label = $3, type_label = $4,
		    due_date = $5, section_id = $6, priority = $7, recur_days = $8,
		    updated_at = now()
		WHERE id = $9 AND owner_user_id = $10`,
		t.Title, t.Description, t.SetupLabel, t.TypeLabel,
		t.DueDate, t.SectionID, t.Priority, t.RecurDays, t.ID, t.OwnerUserID,
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

func (r *TasksRepository) ReplaceLinks(
	ctx context.Context,
	taskID uuid.UUID,
	links []models.TaskLink,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM todos.task_links WHERE task_id = $1`, taskID,
	)
	if err != nil {
		return err
	}
	for i := range links {
		links[i].SortOrder = i
		_, err = r.db.Exec(ctx, `
			INSERT INTO todos.task_links (task_id, url, label, sort_order)
			VALUES ($1, $2, $3, $4)`,
			taskID, links[i].URL, links[i].Label, links[i].SortOrder,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *TasksRepository) AddSubtask(
	ctx context.Context,
	taskID uuid.UUID,
	userID string,
	title string,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO todos.subtasks (task_id, title, sort_order)
		SELECT $1, $2,
		    COALESCE((SELECT MAX(sort_order)+1 FROM todos.subtasks
		              WHERE task_id = $1), 0)
		WHERE EXISTS (
		    SELECT 1 FROM todos.tasks
		    WHERE id = $1 AND owner_user_id = $3
		)`,
		taskID, title, userID,
	)
	return err
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

func (r *TasksRepository) ListSubtasksForTasks(
	ctx context.Context,
	taskIDs []uuid.UUID,
) (map[uuid.UUID][]models.Subtask, error) {
	strIDs := make([]string, len(taskIDs))
	for i, id := range taskIDs {
		strIDs[i] = id.String()
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, title, done, sort_order, created_at
		FROM todos.subtasks
		WHERE task_id = ANY($1::uuid[])
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
			&s.ID, &s.TaskID, &s.Title, &s.Done, &s.SortOrder, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		result[s.TaskID] = append(result[s.TaskID], s)
	}
	return result, rows.Err()
}

func (r *TasksRepository) ListDoneForArchiving(
	ctx context.Context,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.setup_label, t.type_label, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days,
		       COUNT(s.id) FILTER (WHERE s.done)  AS subtask_done,
		       COUNT(s.id)                         AS subtask_total
		FROM todos.tasks t
		JOIN todos.archive_settings a ON a.user_id = t.owner_user_id
		LEFT JOIN todos.subtasks s ON s.task_id = t.id
		WHERE t.status = 'done'
		  AND a.archive_after_hours > 0
		  AND t.completed_at < now() - (a.archive_after_hours * interval '1 hour')
		GROUP BY t.id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *TasksRepository) ArchiveBatch(
	ctx context.Context,
	ids []uuid.UUID,
) error {
	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = id.String()
	}
	now := time.Now()
	_, err := r.db.Exec(ctx, `
		UPDATE todos.tasks
		SET status = 'archived', archived_at = $1, updated_at = now()
		WHERE id = ANY($2::uuid[])`,
		now, strIDs,
	)
	return err
}

func (r *TasksRepository) getLinks(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.TaskLink, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, url, label, sort_order
		FROM todos.task_links
		WHERE task_id = $1
		ORDER BY sort_order`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []models.TaskLink
	for rows.Next() {
		var l models.TaskLink
		if err = rows.Scan(&l.ID, &l.TaskID, &l.URL, &l.Label, &l.SortOrder); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (r *TasksRepository) getSubtasks(
	ctx context.Context,
	taskID uuid.UUID,
) ([]models.Subtask, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, title, done, sort_order, created_at
		FROM todos.subtasks
		WHERE task_id = $1
		ORDER BY sort_order, created_at`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Subtask
	for rows.Next() {
		var s models.Subtask
		if err = rows.Scan(
			&s.ID, &s.TaskID, &s.Title, &s.Done, &s.SortOrder, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func scanTasks(rows pgx.Rows) ([]models.Task, error) {
	var result []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(
			&t.ID, &t.OwnerUserID, &t.Title, &t.Description,
			&t.SetupLabel, &t.TypeLabel, &t.Status,
			&t.Priority, &t.SortOrder,
			&t.CompletedAt, &t.ArchivedAt, &t.DueDate,
			&t.CreatedAt, &t.UpdatedAt, &t.SectionID, &t.WorkspaceID,
			&t.RecurDays, &t.SubtaskDone, &t.SubtaskTotal,
		); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
