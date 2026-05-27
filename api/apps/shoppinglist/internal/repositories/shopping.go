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
	ID     string
	Name   string
	Amount float64
	Unit   string
}

type ShoppingLists struct {
	MealPlanItems []ShoppingItem
	CustomItems   []ShoppingItem
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

func (r *ShoppingRepository) GetShoppingList(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
) (ShoppingLists, error) {
	mealPlanItems, err := r.getMealPlanItems(ctx, planID, start, end)
	if err != nil {
		return ShoppingLists{}, err
	}
	customItems, err := r.getCustomItems(ctx, planID)
	if err != nil {
		return ShoppingLists{}, err
	}
	return ShoppingLists{
		MealPlanItems: mealPlanItems,
		CustomItems:   customItems,
	}, nil
}

func (r *ShoppingRepository) getMealPlanItems(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
) ([]ShoppingItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
		    '' AS id,
		    LOWER(i.name) AS name,
		    i.unit,
		    SUM(i.amount * pm.servings::NUMERIC / r.base_servings::NUMERIC)
		        AS total_amount
		FROM mealplans.plan_meals pm
		JOIN recipes.recipes r ON r.id = pm.recipe_id
		JOIN recipes.ingredients i ON i.recipe_id = r.id
		WHERE pm.plan_id = $1
		  AND pm.meal_date BETWEEN $2 AND $3
		GROUP BY LOWER(i.name), i.unit
		ORDER BY LOWER(i.name)`,
		planID, start, end,
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

func (r *ShoppingRepository) getCustomItems(
	ctx context.Context,
	planID uuid.UUID,
) ([]ShoppingItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ci.id::text, ci.name, ci.unit, ci.amount::float8
		FROM shoppinglist.custom_items ci
		WHERE ci.plan_id = $1
		ORDER BY ci.name`,
		planID,
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
	planID uuid.UUID,
	name, unit string,
	amount float64,
) (ShoppingItem, error) {
	var item ShoppingItem
	err := r.db.QueryRow(ctx, `
		INSERT INTO shoppinglist.custom_items (plan_id, name, amount, unit)
		VALUES ($1, $2, $3, $4)
		RETURNING id::text, name, unit, amount::float8`,
		planID, name, amount, unit,
	).Scan(&item.ID, &item.Name, &item.Unit, &item.Amount)
	if err != nil {
		return ShoppingItem{}, err
	}
	return item, nil
}

func (r *ShoppingRepository) DeleteCustomItem(
	ctx context.Context,
	planID, itemID uuid.UUID,
) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM shoppinglist.custom_items
		WHERE id = $1 AND plan_id = $2`,
		itemID, planID,
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
