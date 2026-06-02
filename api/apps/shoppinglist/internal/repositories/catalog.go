package repositories

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	iapp "tools.xdoubleu.com/internal/app"
)

// ItemName is a distinct, normalized item/ingredient name known for a user,
// together with the category it currently maps to (empty when unassigned).
type ItemName struct {
	Name       string
	CategoryID string
}

// ItemCategory is a raw catalog entry mapping a normalized name to a category.
type ItemCategory struct {
	Name       string
	CategoryID string
}

// ListItemNames returns the distinct normalized names drawn from the user's
// custom items and the ingredients of the recipes they own, each annotated with
// its current category assignment (empty string when none).
func (r *ShoppingRepository) ListItemNames(
	ctx context.Context,
	userID string,
) ([]ItemName, error) {
	rows, err := r.db.Query(ctx, `
		WITH names AS (
		    SELECT DISTINCT LOWER(TRIM(name)) AS name
		    FROM shoppinglist.custom_items
		    WHERE user_id = $1 AND TRIM(name) != ''
		    UNION
		    SELECT DISTINCT LOWER(TRIM(i.name)) AS name
		    FROM recipes.ingredients i
		    JOIN recipes.recipes r ON r.id = i.recipe_id
		    WHERE r.user_id = $1 AND TRIM(i.name) != ''
		)
		SELECT n.name, COALESCE(ic.category_id::text, '')
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
		if err = rows.Scan(&n.Name, &n.CategoryID); err != nil {
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
