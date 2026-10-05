package repositories

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
	"tools.xdoubleu.com/internal/database"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/pagination"
)

type LearningPathsRepository struct {
	db postgres.DB
}

// ListForUser returns a page of the user's learning paths, top-level fields
// only.
func (r *LearningPathsRepository) ListForUser(
	ctx context.Context,
	userID string,
	limit int32,
	offset int32,
) ([]models.LearningPath, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, title, goal, routine, created_at, updated_at, paused
		FROM learningpaths.learning_paths
		WHERE user_id = $1
		ORDER BY title, id
		LIMIT $2 OFFSET $3`,
		userID, sqlLimit, offset,
	)
	if err != nil {
		return nil, false, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var result []models.LearningPath
	for rows.Next() {
		var lp models.LearningPath
		if err = rows.Scan(
			&lp.ID, &lp.UserID, &lp.Title, &lp.Goal, &lp.Routine,
			&lp.CreatedAt, &lp.UpdatedAt, &lp.Paused,
		); err != nil {
			return nil, false, postgres.PgxErrorToHTTPError(err)
		}
		result = append(result, lp)
	}
	if err = rows.Err(); err != nil {
		return nil, false, postgres.PgxErrorToHTTPError(err)
	}

	page, hasMore := pagination.Split(result, safeLimit)
	return page, hasMore, nil
}

func (r *LearningPathsRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*models.LearningPath, error) {
	var lp models.LearningPath
	var schedulesJSON []byte
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, title, goal, routine, created_at, updated_at,
			todoist_project_id, reminder_schedules, paused
		FROM learningpaths.learning_paths
		WHERE id = $1`,
		id,
	).Scan(
		&lp.ID, &lp.UserID, &lp.Title, &lp.Goal, &lp.Routine,
		&lp.CreatedAt, &lp.UpdatedAt, &lp.TodoistProjectID, &schedulesJSON,
		&lp.Paused,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	if err = json.Unmarshal(schedulesJSON, &lp.ReminderSchedules); err != nil {
		return nil, err
	}
	return &lp, nil
}

func (r *LearningPathsRepository) Create(
	ctx context.Context,
	lp models.LearningPath,
) (*models.LearningPath, error) {
	schedulesJSON, err := schedulesToJSON(lp.ReminderSchedules)
	if err != nil {
		return nil, err
	}
	err = r.db.QueryRow(
		ctx,
		`INSERT INTO learningpaths.learning_paths
			(user_id, title, goal, routine, reminder_schedules)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		lp.UserID, lp.Title, lp.Goal, lp.Routine, schedulesJSON,
	).Scan(&lp.ID, &lp.CreatedAt, &lp.UpdatedAt)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &lp, nil
}

func (r *LearningPathsRepository) Update(
	ctx context.Context,
	lp models.LearningPath,
) error {
	schedulesJSON, err := schedulesToJSON(lp.ReminderSchedules)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(
		ctx,
		`UPDATE learningpaths.learning_paths
		SET title = $2, goal = $3, routine = $4, reminder_schedules = $6,
			updated_at = now()
		WHERE id = $1 AND user_id = $5`,
		lp.ID, lp.Title, lp.Goal, lp.Routine, lp.UserID, schedulesJSON,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// schedulesToJSON encodes schedules, storing nil as '{}'.
func schedulesToJSON(schedules map[string]string) ([]byte, error) {
	if schedules == nil {
		schedules = map[string]string{}
	}
	return json.Marshal(schedules)
}

func (r *LearningPathsRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM learningpaths.learning_paths WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// ReplaceModules reconciles a path's modules and items in place: each row is
// matched by the content the client round-trips (title for modules, type and
// description for items), falling back to position, so an edit keeps the IDs —
// and with them recorded progress, Todoist task ids and book links. New rows
// are inserted; rows with no incoming match are deleted (items cascade). A
// matched item's completed flag is never overwritten: progress belongs to
// RecordItemProgress.
func (r *LearningPathsRepository) ReplaceModules(
	ctx context.Context,
	learningPathID uuid.UUID,
	modules []models.Module,
) error {
	existing, err := r.GetModules(ctx, learningPathID)
	if err != nil {
		return err
	}

	used := make([]bool, len(existing))
	for i := range modules {
		quizJSON, marshalErr := json.Marshal(modules[i].Quiz)
		if marshalErr != nil {
			return marshalErr
		}

		var (
			moduleID uuid.UUID
			oldItems []models.Item
		)
		if k := matchModule(existing, used, i, &modules[i]); k >= 0 {
			used[k] = true
			moduleID = existing[k].ID
			oldItems = existing[k].Items
			_, err = r.db.Exec(ctx, `
				UPDATE learningpaths.modules
				SET title = $2, sort_order = $3, quiz = $4
				WHERE id = $1`,
				moduleID, modules[i].Title, i, quizJSON,
			)
		} else {
			err = r.db.QueryRow(ctx, `
				INSERT INTO learningpaths.modules (learning_path_id, title, sort_order, quiz)
				VALUES ($1, $2, $3, $4)
				RETURNING id`,
				learningPathID, modules[i].Title, i, quizJSON,
			).Scan(&moduleID)
		}
		if err != nil {
			return postgres.PgxErrorToHTTPError(err)
		}
		modules[i].ID = moduleID
		modules[i].LearningPathID = learningPathID

		if err = replaceModuleItems(
			ctx, r.db, moduleID, oldItems, modules[i].Items,
		); err != nil {
			return err
		}
	}

	// Delete modules with no incoming match; their items cascade.
	for i, m := range existing {
		if !used[i] {
			if _, err = r.db.Exec(ctx,
				`DELETE FROM learningpaths.modules WHERE id = $1`, m.ID,
			); err != nil {
				return postgres.PgxErrorToHTTPError(err)
			}
		}
	}
	return nil
}

// matchModule finds the existing module the incoming one continues: the same
// title first (the stable key the client round-trips), then the unused module
// at the same position, so a removed module's row is not taken over by the
// one that shifts into its place.
func matchModule(existing []models.Module, used []bool, pos int, m *models.Module) int {
	for i := range existing {
		if !used[i] && existing[i].Title == m.Title {
			return i
		}
	}
	if pos < len(existing) && !used[pos] {
		return pos
	}
	return -1
}

// replaceModuleItems reconciles a module's items: matched rows are updated in
// place, keeping their IDs and completed flags; new items are inserted with
// the caller's flag; unmatched old rows are deleted.
func replaceModuleItems(
	ctx context.Context,
	db postgres.DB,
	moduleID uuid.UUID,
	oldItems, newItems []models.Item,
) error {
	used := make([]bool, len(oldItems))
	for j := range newItems {
		item := &newItems[j]
		if k := matchItem(oldItems, used, j, item); k >= 0 {
			used[k] = true
			_, err := db.Exec(ctx, `
				UPDATE learningpaths.items
				SET type = $2, description = $3, sort_order = $4,
					linked_book_id = $5, due = $6
				WHERE id = $1`,
				oldItems[k].ID, item.Type, item.Description, j,
				item.LinkedBookID, item.Due,
			)
			if err != nil {
				return postgres.PgxErrorToHTTPError(err)
			}
			item.ID = oldItems[k].ID
			item.ModuleID = moduleID
			continue
		}

		err := db.QueryRow(ctx, `
			INSERT INTO learningpaths.items
			(module_id, type, description, sort_order, completed,
			linked_book_id, due)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id`,
			moduleID, item.Type, item.Description, j, item.Completed,
			item.LinkedBookID, item.Due,
		).Scan(&item.ID)
		if err != nil {
			return postgres.PgxErrorToHTTPError(err)
		}
		item.ModuleID = moduleID
	}

	for i, old := range oldItems {
		if !used[i] {
			if _, err := db.Exec(ctx,
				`DELETE FROM learningpaths.items WHERE id = $1`, old.ID,
			); err != nil {
				return postgres.PgxErrorToHTTPError(err)
			}
		}
	}
	return nil
}

// matchItem finds the old item the incoming one continues: same type and
// description first (the stable key the client round-trips), then the unused
// item at the same position.
func matchItem(oldItems []models.Item, used []bool, pos int, item *models.Item) int {
	for i := range oldItems {
		if !used[i] && oldItems[i].Type == item.Type &&
			oldItems[i].Description == item.Description {
			return i
		}
	}
	if pos < len(oldItems) && !used[pos] {
		return pos
	}
	return -1
}

func (r *LearningPathsRepository) GetModules(
	ctx context.Context,
	learningPathID uuid.UUID,
) ([]models.Module, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, learning_path_id, title, sort_order, quiz
		FROM learningpaths.modules
		WHERE learning_path_id = $1
		ORDER BY sort_order`,
		learningPathID,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var modules []models.Module
	for rows.Next() {
		var m models.Module
		var quizJSON []byte
		if err = rows.Scan(
			&m.ID, &m.LearningPathID, &m.Title, &m.SortOrder, &quizJSON,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		if len(quizJSON) > 0 {
			if jsonErr := json.Unmarshal(quizJSON, &m.Quiz); jsonErr != nil {
				return nil, jsonErr
			}
		}
		modules = append(modules, m)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}

	items, err := r.getItemsForPath(ctx, learningPathID)
	if err != nil {
		return nil, err
	}
	for i := range modules {
		modules[i].Items = items[modules[i].ID]
	}

	return modules, nil
}

// getItemsForPath fetches all of a path's items in one query, by module_id.
func (r *LearningPathsRepository) getItemsForPath(
	ctx context.Context,
	learningPathID uuid.UUID,
) (map[uuid.UUID][]models.Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.module_id, i.type, i.description, i.sort_order, i.completed,
			i.linked_book_id, i.todoist_task_id, i.due
		FROM learningpaths.items i
		JOIN learningpaths.modules m ON m.id = i.module_id
		WHERE m.learning_path_id = $1
		ORDER BY i.sort_order`,
		learningPathID,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	result := map[uuid.UUID][]models.Item{}
	for rows.Next() {
		var item models.Item
		if err = rows.Scan(
			&item.ID, &item.ModuleID, &item.Type, &item.Description,
			&item.SortOrder, &item.Completed, &item.LinkedBookID, &item.TodoistTaskID,
			&item.Due,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		result[item.ModuleID] = append(result[item.ModuleID], item)
	}
	if err = rows.Err(); err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return result, nil
}

func (r *LearningPathsRepository) ReplaceResources(
	ctx context.Context,
	learningPathID uuid.UUID,
	resources []models.Resource,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM learningpaths.resources WHERE learning_path_id = $1`,
		learningPathID,
	)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	if len(resources) == 0 {
		return nil
	}

	//nolint:exhaustruct //other fields optional
	batch := &pgx.Batch{}
	for i, resource := range resources {
		batch.Queue(`
			INSERT INTO learningpaths.resources
			(learning_path_id, text, sort_order, linked_book_id, linked_feed_item_id)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id`,
			learningPathID, resource.Text, i,
			resource.LinkedBookID, resource.LinkedFeedItemID,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	for i := range resources {
		var resourceID uuid.UUID
		if err = br.QueryRow().Scan(&resourceID); err != nil {
			_ = br.Close()
			return postgres.PgxErrorToHTTPError(err)
		}
		resources[i].ID = resourceID
		resources[i].LearningPathID = learningPathID
	}
	return postgres.PgxErrorToHTTPError(br.Close())
}

func (r *LearningPathsRepository) GetResources(
	ctx context.Context,
	learningPathID uuid.UUID,
) ([]models.Resource, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, learning_path_id, text, sort_order,
			linked_book_id, linked_feed_item_id
		FROM learningpaths.resources
		WHERE learning_path_id = $1
		ORDER BY sort_order`,
		learningPathID,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	defer rows.Close()

	var resources []models.Resource
	for rows.Next() {
		var res models.Resource
		if err = rows.Scan(
			&res.ID, &res.LearningPathID, &res.Text, &res.SortOrder,
			&res.LinkedBookID, &res.LinkedFeedItemID,
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		resources = append(resources, res)
	}
	return resources, postgres.PgxErrorToHTTPError(rows.Err())
}

// RecordItemProgress sets an item's completed flag, scoped to userID's paths.
// Returns database.ErrResourceNotFound when not found or not owned.
func (r *LearningPathsRepository) RecordItemProgress(
	ctx context.Context,
	itemID uuid.UUID,
	userID string,
	completed bool,
) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE learningpaths.items
		SET completed = $1
		WHERE id = $2
		AND module_id IN (
			SELECT m.id
			FROM learningpaths.modules m
			JOIN learningpaths.learning_paths lp ON lp.id = m.learning_path_id
			WHERE lp.user_id = $3
		)`,
		completed, itemID, userID,
	)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrResourceNotFound
	}
	return nil
}

// GetItemForUser returns an owned item's description/type plus its path's
// title (404 on foreign ownership).
func (r *LearningPathsRepository) GetItemForUser(
	ctx context.Context, itemID uuid.UUID, userID string,
) (*models.ItemForTask, error) {
	var out models.ItemForTask
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.module_id, i.type, i.description, i.sort_order,
		       i.completed, i.linked_book_id, i.due, lp.title
		FROM learningpaths.items i
		JOIN learningpaths.modules m ON m.id = i.module_id
		JOIN learningpaths.learning_paths lp ON lp.id = m.learning_path_id
		WHERE i.id = $1 AND lp.user_id = $2
	`, itemID, userID).Scan(
		&out.Item.ID, &out.Item.ModuleID, &out.Item.Type, &out.Item.Description,
		&out.Item.SortOrder, &out.Item.Completed, &out.Item.LinkedBookID,
		&out.Item.Due, &out.PathTitle,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &out, nil
}

// GetPathIDForItem returns the id of the path that owns itemID for userID,
// scoped so a foreign item reads as not found.
func (r *LearningPathsRepository) GetPathIDForItem(
	ctx context.Context, itemID uuid.UUID, userID string,
) (uuid.UUID, error) {
	var pathID uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT m.learning_path_id
		FROM learningpaths.items i
		JOIN learningpaths.modules m ON m.id = i.module_id
		JOIN learningpaths.learning_paths lp ON lp.id = m.learning_path_id
		WHERE i.id = $1 AND lp.user_id = $2
	`, itemID, userID).Scan(&pathID)
	if err != nil {
		return uuid.UUID{}, postgres.PgxErrorToHTTPError(err)
	}
	return pathID, nil
}

// SetItemTodoistTaskID stores/clears the Todoist task id for an item.
func (r *LearningPathsRepository) SetItemTodoistTaskID(
	ctx context.Context, itemID uuid.UUID, taskID string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE learningpaths.items SET todoist_task_id = $1 WHERE id = $2`,
		taskID, itemID,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// SetTodoistProjectID stores/clears the Todoist project id for a path.
func (r *LearningPathsRepository) SetTodoistProjectID(
	ctx context.Context, pathID uuid.UUID, projectID string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE learningpaths.learning_paths
		SET todoist_project_id = NULLIF($1, '') WHERE id = $2`,
		projectID, pathID,
	)
	return postgres.PgxErrorToHTTPError(err)
}

// SetPaused stores whether userID's path is paused.
func (r *LearningPathsRepository) SetPaused(
	ctx context.Context, id uuid.UUID, userID string, paused bool,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE learningpaths.learning_paths
		SET paused = $3, updated_at = now()
		WHERE id = $1 AND user_id = $2`,
		id, userID, paused,
	)
	return postgres.PgxErrorToHTTPError(err)
}
