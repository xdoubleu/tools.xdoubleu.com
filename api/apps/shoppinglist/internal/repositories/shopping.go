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
	Name   string
	Amount float64
	Unit   string
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
) ([]ShoppingItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
		    LOWER(i.name) AS name,
		    i.unit,
		    SUM(i.amount * pm.servings::NUMERIC / r.base_servings::NUMERIC) AS total_amount
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
		if err = rows.Scan(&item.Name, &item.Unit, &item.Amount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
