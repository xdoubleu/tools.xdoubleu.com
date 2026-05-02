package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/recipes/internal/models"
)

type ShoppingRepository struct {
	db postgres.DB
}

func (r *ShoppingRepository) GetShoppingList(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
) ([]models.ShoppingItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
		    LOWER(i.name) AS name,
		    i.unit,
		    SUM(i.amount * pm.servings::NUMERIC / r.base_servings::NUMERIC) AS total_amount
		FROM recipes.plan_meals pm
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

	var result []models.ShoppingItem
	for rows.Next() {
		var item models.ShoppingItem
		if err = rows.Scan(&item.Name, &item.Unit, &item.Amount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ensure postgres import is used.
var _ = postgres.PgxErrorToHTTPError
