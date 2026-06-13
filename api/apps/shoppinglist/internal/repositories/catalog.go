package repositories

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	iapp "tools.xdoubleu.com/internal/app"
)

// ItemName is a distinct, normalized item/ingredient name known for a user,
// together with the category it currently maps to (empty when unassigned).
// Excluded is true when the name only survives via meal-plan entries flagged
// exclude_from_shopping_list (no active source remains), so the catalog can
// offer to restore it.
type ItemName struct {
	Name       string
	CategoryID string
	Excluded   bool
}

// ItemCategory is a raw catalog entry mapping a normalized name to a category.
type ItemCategory struct {
	Name       string
	CategoryID string
}

// ListItemNames returns the distinct normalized names drawn from the user's
// custom items, the ingredients of the recipes they own, and their meal-plan
// custom entries, each annotated with its current category assignment (empty
// string when none) and whether it is excluded from the export.
//
// A name is reported excluded only when it survives solely through meal-plan
// entries flagged exclude_from_shopping_list and has no active source left
// (custom item, owned recipe ingredient, or an exporting meal-plan entry). Such
// names are surfaced so the UI can offer to restore them.
func (r *ShoppingRepository) ListItemNames(
	ctx context.Context,
	userID string,
) ([]ItemName, error) {
	rows, err := r.db.Query(ctx, `
		WITH active AS (
		    SELECT DISTINCT LOWER(TRIM(name)) AS name
		    FROM shoppinglist.custom_items
		    WHERE user_id = $1 AND TRIM(name) != ''
		    UNION
		    SELECT DISTINCT LOWER(TRIM(i.name)) AS name
		    FROM recipes.ingredients i
		    JOIN recipes.recipes r ON r.id = i.recipe_id
		    WHERE r.user_id = $1 AND TRIM(i.name) != ''
		    UNION
		    -- Recipe-less meal entries store hand-typed items as a
		    -- newline-separated list in custom_name (each line a bare "name" or
		    -- "name\tamount"); surface those names so they can be categorized.
		    SELECT DISTINCT LOWER(TRIM(split_part(item, E'\t', 1))) AS name
		    FROM mealplans.plan_meals pm
		    JOIN mealplans.plans p ON p.id = pm.plan_id,
		         unnest(string_to_array(pm.custom_name, E'\n')) AS item
		    WHERE pm.recipe_id IS NULL
		      AND pm.exclude_from_shopping_list = FALSE
		      AND (p.owner_user_id = $1
		           OR p.id IN (
		               SELECT plan_id FROM mealplans.plan_access WHERE user_id = $1
		           ))
		      AND TRIM(split_part(item, E'\t', 1)) != ''
		),
		excluded AS (
		    -- Names that currently only exist on excluded meal-plan entries.
		    SELECT DISTINCT LOWER(TRIM(split_part(item, E'\t', 1))) AS name
		    FROM mealplans.plan_meals pm
		    JOIN mealplans.plans p ON p.id = pm.plan_id,
		         unnest(string_to_array(pm.custom_name, E'\n')) AS item
		    WHERE pm.recipe_id IS NULL
		      AND pm.exclude_from_shopping_list = TRUE
		      AND (p.owner_user_id = $1
		           OR p.id IN (
		               SELECT plan_id FROM mealplans.plan_access WHERE user_id = $1
		           ))
		      AND TRIM(split_part(item, E'\t', 1)) != ''
		),
		names AS (
		    SELECT name, FALSE AS excluded FROM active
		    UNION ALL
		    SELECT name, TRUE AS excluded FROM excluded
		    WHERE name NOT IN (SELECT name FROM active)
		)
		SELECT n.name, COALESCE(ic.category_id::text, ''), n.excluded
		FROM names n
		LEFT JOIN shoppinglist.item_categories ic
		    ON ic.user_id = $1 AND ic.name = n.name
		ORDER BY n.name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ItemName
	for rows.Next() {
		var n ItemName
		if err = rows.Scan(&n.Name, &n.CategoryID, &n.Excluded); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

// ListItemCategories returns the raw name -> category map for a user.
func (r *ShoppingRepository) ListItemCategories(
	ctx context.Context,
	userID string,
) ([]ItemCategory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT name, category_id::text
		FROM shoppinglist.item_categories
		WHERE user_id = $1
		ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ItemCategory
	for rows.Next() {
		var ic ItemCategory
		if err = rows.Scan(&ic.Name, &ic.CategoryID); err != nil {
			return nil, err
		}
		result = append(result, ic)
	}
	return result, rows.Err()
}

// SetItemCategory assigns categoryID to name (normalized). When categoryID is
// uuid.Nil the mapping is removed instead.
func (r *ShoppingRepository) SetItemCategory(
	ctx context.Context,
	userID, name string,
	categoryID uuid.UUID,
) error {
	if categoryID == uuid.Nil {
		_, err := r.db.Exec(ctx, `
			DELETE FROM shoppinglist.item_categories
			WHERE user_id = $1 AND name = LOWER(TRIM($2))`,
			userID, name,
		)
		return err
	}

	result, err := r.db.Exec(ctx, `
		INSERT INTO shoppinglist.item_categories (user_id, name, category_id)
		SELECT $1, LOWER(TRIM($2)), c.id
		FROM shoppinglist.categories c
		WHERE c.id = $3 AND c.user_id = $1
		ON CONFLICT (user_id, name)
		DO UPDATE SET category_id = excluded.category_id`,
		userID, name, categoryID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Category not found",
		}
	}
	return nil
}

// SetItemExcluded removes a normalized name from the export (excluded=true) or
// restores it (excluded=false). Removing flips exclude_from_shopping_list on the
// caller's matching meal-plan custom entries and deletes matching shopping-list
// custom items; restoring only clears the flag (deleted custom items cannot be
// restored). Recipe ingredients are never touched. The meal-plan update is
// limited to plans the user owns or can edit.
func (r *ShoppingRepository) SetItemExcluded(
	ctx context.Context,
	userID, name string,
	excluded bool,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE mealplans.plan_meals pm
		SET exclude_from_shopping_list = $3
		FROM mealplans.plans p
		WHERE pm.plan_id = p.id
		  AND pm.recipe_id IS NULL
		  AND (p.owner_user_id = $1
		       OR EXISTS (
		           SELECT 1 FROM mealplans.plan_access pa
		           WHERE pa.plan_id = p.id
		             AND pa.user_id = $1
		             AND pa.can_edit
		       ))
		  AND EXISTS (
		      SELECT 1
		      FROM unnest(string_to_array(pm.custom_name, E'\n')) AS item
		      WHERE LOWER(TRIM(split_part(item, E'\t', 1))) = LOWER(TRIM($2))
		  )`,
		userID, name, excluded,
	)
	if err != nil {
		return err
	}

	if !excluded {
		return nil
	}

	_, err = r.db.Exec(ctx, `
		DELETE FROM shoppinglist.custom_items
		WHERE user_id = $1 AND LOWER(TRIM(name)) = LOWER(TRIM($2))`,
		userID, name,
	)
	return err
}
