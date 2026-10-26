package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrSelfAlias is returned when an item would be merged into itself.
var ErrSelfAlias = errors.New("can't merge an item into itself")

// IngredientUnit is a unit the family's recipes use for an alias-resolved
// ingredient name, with the recipes using it.
type IngredientUnit struct {
	Name      string
	Unit      string
	RecipeIDs []string
}

// SetItemAlias exports alias as canonical, both already normalized. A
// canonical that is itself an alias resolves to its own canonical, and
// aliases of alias move along, so chains stay one level deep. canonical
// inherits alias's category when it has none.
func (r *ShoppingRepository) SetItemAlias(
	ctx context.Context,
	familyID uuid.UUID,
	alias, canonical string,
) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(
			    (SELECT canonical FROM shoppinglist.item_aliases
			     WHERE family_id = $1 AND alias = $2),
			    $2
			)`,
			familyID, canonical,
		).Scan(&canonical)
		if err != nil {
			return err
		}
		if canonical == alias {
			return ErrSelfAlias
		}

		if _, err = tx.Exec(ctx, `
			UPDATE shoppinglist.item_aliases
			SET canonical = $3
			WHERE family_id = $1 AND canonical = $2`,
			familyID, alias, canonical,
		); err != nil {
			return err
		}

		if _, err = tx.Exec(ctx, `
			INSERT INTO shoppinglist.item_aliases (family_id, alias, canonical)
			VALUES ($1, $2, $3)
			ON CONFLICT (family_id, alias)
			DO UPDATE SET canonical = excluded.canonical`,
			familyID, alias, canonical,
		); err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO shoppinglist.item_categories (family_id, name, category_id)
			SELECT family_id, $3, category_id
			FROM shoppinglist.item_categories
			WHERE family_id = $1 AND name = $2
			ON CONFLICT (family_id, name) DO NOTHING`,
			familyID, alias, canonical,
		)
		return err
	})
}

// DeleteItemAlias unmerges alias (normalized).
func (r *ShoppingRepository) DeleteItemAlias(
	ctx context.Context,
	familyID uuid.UUID,
	alias string,
) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM shoppinglist.item_aliases
		WHERE family_id = $1 AND alias = $2`,
		familyID, alias,
	)
	return err
}

// ListIngredientUnits returns, per alias-resolved ingredient name and unit,
// the family's recipes using that pair.
func (r *ShoppingRepository) ListIngredientUnits(
	ctx context.Context,
	familyID uuid.UUID,
) ([]IngredientUnit, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
		    COALESCE(a.canonical, LOWER(TRIM(i.name))) AS name,
		    TRIM(i.unit) AS unit,
		    array_agg(DISTINCT r.id::text ORDER BY r.id::text) AS recipe_ids
		FROM recipes.ingredients i
		JOIN recipes.recipes r ON r.id = i.recipe_id
		LEFT JOIN shoppinglist.item_aliases a
		    ON a.family_id = r.family_id AND a.alias = LOWER(TRIM(i.name))
		WHERE r.family_id = $1 AND TRIM(i.name) != ''
		GROUP BY COALESCE(a.canonical, LOWER(TRIM(i.name))), TRIM(i.unit)
		ORDER BY 1, 2`,
		familyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []IngredientUnit
	for rows.Next() {
		var u IngredientUnit
		if err = rows.Scan(&u.Name, &u.Unit, &u.RecipeIDs); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}
