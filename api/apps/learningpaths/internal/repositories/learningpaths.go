package repositories

import (
	"context"

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

// ListForUser returns a page of the user's learning paths. The tree
// (modules/items/resources) is not populated here — List only needs the
// top-level fields, matching recipes.RecipesRepository.ListForFamily.
func (r *LearningPathsRepository) ListForUser(
	ctx context.Context,
	userID string,
	limit int32,
	offset int32,
) ([]models.LearningPath, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, title, goal, routine, created_at, updated_at
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
			&lp.CreatedAt, &lp.UpdatedAt,
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
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, title, goal, routine, created_at, updated_at
		FROM learningpaths.learning_paths
		WHERE id = $1`,
		id,
	).Scan(
		&lp.ID, &lp.UserID, &lp.Title, &lp.Goal, &lp.Routine,
		&lp.CreatedAt, &lp.UpdatedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &lp, nil
}

func (r *LearningPathsRepository) Create(
	ctx context.Context,
	lp models.LearningPath,
) (*models.LearningPath, error) {
	err := r.db.QueryRow(
		ctx,
		`INSERT INTO learningpaths.learning_paths (user_id, title, goal, routine)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`,
		lp.UserID, lp.Title, lp.Goal, lp.Routine,
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
	_, err := r.db.Exec(
		ctx,
		`UPDATE learningpaths.learning_paths
		SET title = $2, goal = $3, routine = $4, updated_at = now()
		WHERE id = $1 AND user_id = $5`,
		lp.ID, lp.Title, lp.Goal, lp.Routine, lp.UserID,
	)
	return postgres.PgxErrorToHTTPError(err)
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

// ReplaceModules wholesale-replaces a path's modules (and, via cascade,
// their items) — the same delete-all-then-reinsert pattern
// recipes.RecipesRepository.ReplaceIngredients uses. Each Item's Completed
// flag is whatever the caller passes in, so day-to-day progress toggling
// should go through RecordItemProgress instead of a full tree resend.
func (r *LearningPathsRepository) ReplaceModules(
	ctx context.Context,
	learningPathID uuid.UUID,
	modules []models.Module,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM learningpaths.modules WHERE learning_path_id = $1`,
		learningPathID,
	)
	if err != nil {
		return postgres.PgxErrorToHTTPError(err)
	}

	if len(modules) == 0 {
		return nil
	}

	for i := range modules {
		var moduleID uuid.UUID
		err = r.db.QueryRow(ctx,
			`INSERT INTO learningpaths.modules (learning_path_id, title, sort_order)
			VALUES ($1, $2, $3)
			RETURNING id`,
			learningPathID, modules[i].Title, i,
		).Scan(&moduleID)
		if err != nil {
			return postgres.PgxErrorToHTTPError(err)
		}
		modules[i].ID = moduleID
		modules[i].LearningPathID = learningPathID

		if len(modules[i].Items) == 0 {
			continue
		}

		//nolint:exhaustruct //other fields optional
		batch := &pgx.Batch{}
		for j, item := range modules[i].Items {
			batch.Queue(`
				INSERT INTO learningpaths.items
				(module_id, type, description, sort_order, completed)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id`,
				moduleID, item.Type, item.Description, j, item.Completed,
			)
		}

		br := r.db.SendBatch(ctx, batch)
		for j := range modules[i].Items {
			var itemID uuid.UUID
			if err = br.QueryRow().Scan(&itemID); err != nil {
				_ = br.Close()
				return postgres.PgxErrorToHTTPError(err)
			}
			modules[i].Items[j].ID = itemID
			modules[i].Items[j].ModuleID = moduleID
		}
		if err = br.Close(); err != nil {
			return postgres.PgxErrorToHTTPError(err)
		}
	}

	return nil
}

func (r *LearningPathsRepository) GetModules(
	ctx context.Context,
	learningPathID uuid.UUID,
) ([]models.Module, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, learning_path_id, title, sort_order
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
		if err = rows.Scan(&m.ID, &m.LearningPathID, &m.Title, &m.SortOrder); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
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

// getItemsForPath fetches every item across every module of a path in one
// query, grouped by module_id — avoids an N+1 per module.
func (r *LearningPathsRepository) getItemsForPath(
	ctx context.Context,
	learningPathID uuid.UUID,
) (map[uuid.UUID][]models.Item, error) {
	rows, err := r.db.Query(ctx, `
		SELECT i.id, i.module_id, i.type, i.description, i.sort_order, i.completed
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
			&item.SortOrder, &item.Completed,
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
			INSERT INTO learningpaths.resources (learning_path_id, text, sort_order)
			VALUES ($1, $2, $3)
			RETURNING id`,
			learningPathID, resource.Text, i,
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
		SELECT id, learning_path_id, text, sort_order
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
		); err != nil {
			return nil, postgres.PgxErrorToHTTPError(err)
		}
		resources = append(resources, res)
	}
	return resources, postgres.PgxErrorToHTTPError(rows.Err())
}

// RecordItemProgress updates a single item's completed flag, scoped by the
// owning path's user_id via a join so a caller can never toggle another
// user's item. Returns database.ErrResourceNotFound when the item doesn't
// exist or isn't owned by userID.
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

// GetItemForUser returns itemID's description/type plus its owning path's
// title, scoped by userID (404 on foreign ownership, same rule as every
// other per-user lookup in this app) — used by SendItemToTodoist to build a
// task's content without pulling the whole path tree.
func (r *LearningPathsRepository) GetItemForUser(
	ctx context.Context, itemID uuid.UUID, userID string,
) (*models.ItemForTask, error) {
	var out models.ItemForTask
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.module_id, i.type, i.description, i.sort_order,
		       i.completed, lp.title
		FROM learningpaths.items i
		JOIN learningpaths.modules m ON m.id = i.module_id
		JOIN learningpaths.learning_paths lp ON lp.id = m.learning_path_id
		WHERE i.id = $1 AND lp.user_id = $2
	`, itemID, userID).Scan(
		&out.Item.ID, &out.Item.ModuleID, &out.Item.Type, &out.Item.Description,
		&out.Item.SortOrder, &out.Item.Completed, &out.PathTitle,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &out, nil
}
