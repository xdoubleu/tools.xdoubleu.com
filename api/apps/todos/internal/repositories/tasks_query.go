package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

func (r *TasksRepository) ListOpen(
	ctx context.Context,
	userID string,
	sectionID *uuid.UUID,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.labels, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date, t.deadline,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days, t.recur_rule,
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

func (r *TasksRepository) CountOpenPerSection(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `
		SELECT section_id, COUNT(*) AS cnt
		FROM todos.tasks
		WHERE owner_user_id = $1 AND status = 'open'
		  AND (workspace_id = $2 OR ($2::uuid IS NULL AND workspace_id IS NULL))
		GROUP BY section_id`,
		userID, workspaceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var sectionID *uuid.UUID
		var cnt int
		if scanErr := rows.Scan(&sectionID, &cnt); scanErr != nil {
			return nil, scanErr
		}
		key := "open"
		if sectionID != nil {
			key = sectionID.String()
		}
		counts[key] = cnt
	}
	return counts, rows.Err()
}

func (r *TasksRepository) ListByStatus(
	ctx context.Context,
	userID string,
	status string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.labels, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date, t.deadline,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days, t.recur_rule,
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
		       t.labels, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date, t.deadline,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days, t.recur_rule,
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
		       t.labels, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date, t.deadline,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days, t.recur_rule,
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

// SearchByLinkURL finds tasks that have a link whose URL matches the given value.
func (r *TasksRepository) SearchByLinkURL(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
	linkURL string,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.labels, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date, t.deadline,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days, t.recur_rule,
		       COUNT(s.id) FILTER (WHERE s.done)  AS subtask_done,
		       COUNT(s.id)                         AS subtask_total
		FROM todos.tasks t
		JOIN todos.task_links l ON l.task_id = t.id
		LEFT JOIN todos.subtasks s ON s.task_id = t.id
		WHERE t.owner_user_id = $1
		  AND (t.workspace_id = $2 OR ($2::uuid IS NULL AND t.workspace_id IS NULL))
		  AND l.url = $3
		GROUP BY t.id
		ORDER BY t.status ASC, t.created_at DESC`,
		userID, workspaceID, linkURL,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

func (r *TasksRepository) ListDoneForArchiving(
	ctx context.Context,
) ([]models.Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.owner_user_id, t.title, t.description,
		       t.labels, t.status, t.priority, t.sort_order,
		       t.completed_at, t.archived_at, t.due_date, t.deadline,
		       t.created_at, t.updated_at, t.section_id, t.workspace_id,
		       t.recur_days, t.recur_rule,
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

// ListLinksForTasks batch-fetches task links for a set of task IDs.
func (r *TasksRepository) ListLinksForTasks(
	ctx context.Context,
	taskIDs []uuid.UUID,
) (map[uuid.UUID][]models.TaskLink, error) {
	strIDs := make([]string, len(taskIDs))
	for i, id := range taskIDs {
		strIDs[i] = id.String()
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, task_id, url, label, sort_order
		FROM todos.task_links
		WHERE task_id = ANY($1::uuid[])
		ORDER BY task_id, sort_order`,
		strIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]models.TaskLink)
	for rows.Next() {
		var l models.TaskLink
		if err = rows.Scan(
			&l.ID, &l.TaskID, &l.URL, &l.Label, &l.SortOrder,
		); err != nil {
			return nil, err
		}
		result[l.TaskID] = append(result[l.TaskID], l)
	}
	return result, rows.Err()
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
		WITH RECURSIVE sub_tree AS (
		  SELECT id, task_id, parent_subtask_id, title, description, done,
		         sort_order, priority, labels, due_date, deadline, created_at,
		         updated_at
		  FROM todos.subtasks
		  WHERE task_id = $1 AND parent_subtask_id IS NULL
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
			&s.ID, &s.TaskID, &s.ParentSubtaskID, &s.Title, &s.Description,
			&s.Done, &s.SortOrder, &s.Priority, &s.Labels, &s.DueDate,
			&s.Deadline, &s.CreatedAt, &s.UpdatedAt,
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
			&t.Labels, &t.Status,
			&t.Priority, &t.SortOrder,
			&t.CompletedAt, &t.ArchivedAt, &t.DueDate, &t.Deadline,
			&t.CreatedAt, &t.UpdatedAt, &t.SectionID, &t.WorkspaceID,
			&t.RecurDays, &t.RecurRule, &t.SubtaskDone, &t.SubtaskTotal,
		); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
