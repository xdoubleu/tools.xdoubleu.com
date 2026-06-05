package repositories

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"

	iapp "tools.xdoubleu.com/internal/app"
)

type ShoppingRepository struct {
	db postgres.DB
}

func New(db postgres.DB) *ShoppingRepository {
	return &ShoppingRepository{db: db}
}

type ShoppingItem struct {
	ID         string
	Name       string
	Amount     float64
	Unit       string
	RecipeName string
	GroupName  string
}

// CheckPlanAccess returns an error if userID cannot access planID.
func (r *ShoppingRepository) CheckPlanAccess(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
) error {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM mealplans.plans p
			WHERE p.id = $1
			  AND (p.owner_user_id = $2
			       OR p.id IN (
			           SELECT plan_id FROM mealplans.plan_access WHERE user_id = $2
			       ))
		)`,
		planID, userID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Plan not found",
		}
	}
	return nil
}

func (r *ShoppingRepository) GetCustomItems(
	ctx context.Context,
	userID string,
) ([]ShoppingItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ci.id::text, ci.name, ci.unit, ci.amount::float8
		FROM shoppinglist.custom_items ci
		WHERE ci.user_id = $1
		ORDER BY ci.name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ShoppingItem
	for rows.Next() {
		var item ShoppingItem
		if err = rows.Scan(&item.ID, &item.Name, &item.Unit, &item.Amount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *ShoppingRepository) AddCustomItem(
	ctx context.Context,
	userID, name, unit string,
	amount float64,
) (ShoppingItem, error) {
	var item ShoppingItem
	err := r.db.QueryRow(ctx, `
		INSERT INTO shoppinglist.custom_items (user_id, name, amount, unit)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, name, unit, amount::float8`,
		userID, name, amount, unit,
	).Scan(&item.ID, &item.Name, &item.Unit, &item.Amount)
	if err != nil {
		return ShoppingItem{}, err
	}
	return item, nil
}

func (r *ShoppingRepository) DeleteCustomItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM shoppinglist.custom_items
		WHERE id = $1 AND user_id = $2`,
		itemID, userID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return &iapp.HTTPError{
			Status:  http.StatusNotFound,
			Message: "Item not found",
		}
	}
	return nil
}

func (r *ShoppingRepository) GetMealPlanExportItems(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
	pastSlots []string,
	excludedGroups []string,
) ([]ShoppingItem, error) {
	// pgx sends nil slices as SQL NULL; normalize to empty arrays so that
	// ANY($n::text[]) never evaluates to NULL and accidentally excludes rows.
	if pastSlots == nil {
		pastSlots = []string{}
	}
	if excludedGroups == nil {
		excludedGroups = []string{}
	}
	rows, err := r.db.Query(ctx, `
		WITH recipe_effective_servings AS (
		    SELECT
		        r.id AS recipe_id,
		        r.name AS recipe_name,
		        COALESCE(r.batch_servings, SUM(pm.servings))::NUMERIC
		            AS effective_servings,
		        r.base_servings::NUMERIC
		    FROM mealplans.plan_meals pm
		    JOIN recipes.recipes r ON r.id = pm.recipe_id
		    WHERE pm.plan_id = $1
		      AND pm.meal_date BETWEEN $2 AND $3
		      AND NOT (pm.meal_date = $2 AND pm.meal_slot = ANY($4::text[]))
		    GROUP BY r.id, r.name, r.batch_servings, r.base_servings
		)
		SELECT
		    res.recipe_name,
		    COALESCE(i.group_name, '') AS group_name,
		    LOWER(i.name) AS name,
		    i.unit,
		    SUM(i.amount * res.effective_servings / res.base_servings)
		        AS total_amount
		FROM recipe_effective_servings res
		JOIN recipes.ingredients i ON i.recipe_id = res.recipe_id
		WHERE i.group_name IS NULL OR NOT (i.group_name = ANY($5::text[]))
		GROUP BY res.recipe_name, i.group_name, LOWER(i.name), i.unit
		ORDER BY res.recipe_name, COALESCE(i.group_name, ''), LOWER(i.name)`,
		planID, start, end, pastSlots, excludedGroups,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ShoppingItem
	for rows.Next() {
		var item ShoppingItem
		if err = rows.Scan(
			&item.RecipeName, &item.GroupName, &item.Name, &item.Unit, &item.Amount,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type PlanIngredientGroup struct {
	RecipeName string
	GroupName  string
}

func (r *ShoppingRepository) GetPlanIngredientGroups(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
	pastSlots []string,
) ([]PlanIngredientGroup, error) {
	if pastSlots == nil {
		pastSlots = []string{}
	}
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT r.name AS recipe_name, i.group_name
		FROM mealplans.plan_meals pm
		JOIN recipes.recipes r ON r.id = pm.recipe_id
		JOIN recipes.ingredients i ON i.recipe_id = r.id
		WHERE pm.plan_id = $1
		  AND pm.meal_date BETWEEN $2 AND $3
		  AND NOT (pm.meal_date = $2 AND pm.meal_slot = ANY($4::text[]))
		  AND i.group_name IS NOT NULL
		ORDER BY r.name, i.group_name`,
		planID, start, end, pastSlots,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PlanIngredientGroup
	for rows.Next() {
		var g PlanIngredientGroup
		if err = rows.Scan(&g.RecipeName, &g.GroupName); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}
