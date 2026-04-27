package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/xdoubleu/essentia/v3/pkg/database/postgres"
	"tools.xdoubleu.com/apps/recipes/internal/models"
)

type PlansRepository struct {
	db postgres.DB
}

func (r *PlansRepository) ListForUser(
	ctx context.Context,
	userID string,
) ([]models.Plan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.owner_user_id, p.name, p.start_date, p.end_date,
		       p.ical_token, p.created_at, p.updated_at,
		       COALESCE(pa.can_edit, p.owner_user_id = $1) AS can_edit
		FROM recipes.plans p
		LEFT JOIN recipes.plan_access pa ON pa.plan_id = p.id AND pa.user_id = $1
		WHERE p.owner_user_id = $1 OR pa.user_id = $1
		ORDER BY p.start_date DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Plan
	for rows.Next() {
		var plan models.Plan
		if err = rows.Scan(
			&plan.ID, &plan.OwnerUserID, &plan.Name, &plan.StartDate, &plan.EndDate,
			&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt, &plan.CanEdit,
		); err != nil {
			return nil, err
		}
		result = append(result, plan)
	}
	return result, rows.Err()
}

func (r *PlansRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Plan, error) {
	var plan models.Plan
	err := r.db.QueryRow(ctx, `
		SELECT p.id, p.owner_user_id, p.name, p.start_date, p.end_date,
		       p.ical_token, p.created_at, p.updated_at,
		       COALESCE(pa.can_edit, p.owner_user_id = $2) AS can_edit
		FROM recipes.plans p
		LEFT JOIN recipes.plan_access pa ON pa.plan_id = p.id AND pa.user_id = $2
		WHERE p.id = $1 AND (p.owner_user_id = $2 OR pa.user_id = $2)`,
		id, userID,
	).Scan(
		&plan.ID, &plan.OwnerUserID, &plan.Name, &plan.StartDate, &plan.EndDate,
		&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt, &plan.CanEdit,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &plan, nil
}

func (r *PlansRepository) GetByICalToken(
	ctx context.Context,
	token uuid.UUID,
) (*models.Plan, error) {
	var plan models.Plan
	err := r.db.QueryRow(ctx, `
		SELECT id, owner_user_id, name, start_date, end_date,
		       ical_token, created_at, updated_at
		FROM recipes.plans
		WHERE ical_token = $1`,
		token,
	).Scan(
		&plan.ID, &plan.OwnerUserID, &plan.Name, &plan.StartDate, &plan.EndDate,
		&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &plan, nil
}

func (r *PlansRepository) Create(
	ctx context.Context,
	plan models.Plan,
) (*models.Plan, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO recipes.plans (owner_user_id, name, start_date, end_date)
		VALUES ($1, $2, $3, $4)
		RETURNING id, ical_token, created_at, updated_at`,
		plan.OwnerUserID, plan.Name, plan.StartDate, plan.EndDate,
	).Scan(&plan.ID, &plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt)
	if err != nil {
		return nil, err
	}
	plan.CanEdit = true
	return &plan, nil
}

func (r *PlansRepository) Update(
	ctx context.Context,
	plan models.Plan,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE recipes.plans
		SET name = $3, start_date = $4, end_date = $5, updated_at = now()
		WHERE id = $1 AND owner_user_id = $2`,
		plan.ID, plan.OwnerUserID, plan.Name, plan.StartDate, plan.EndDate,
	)
	return err
}

func (r *PlansRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	ownerUserID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.plans WHERE id = $1 AND owner_user_id = $2`,
		id, ownerUserID,
	)
	return err
}

func (r *PlansRepository) AddMeal(
	ctx context.Context,
	meal models.PlanMeal,
) (*models.PlanMeal, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO recipes.plan_meals (plan_id, meal_date, meal_slot, recipe_id, servings)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (plan_id, meal_date, meal_slot)
		DO UPDATE SET recipe_id = EXCLUDED.recipe_id, servings = EXCLUDED.servings
		RETURNING id`,
		meal.PlanID, meal.MealDate, meal.MealSlot, meal.RecipeID, meal.Servings,
	).Scan(&meal.ID)
	if err != nil {
		return nil, err
	}
	return &meal, nil
}

func (r *PlansRepository) DeleteMeal(
	ctx context.Context,
	mealID uuid.UUID,
	planID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.plan_meals WHERE id = $1 AND plan_id = $2`,
		mealID, planID,
	)
	return err
}

func (r *PlansRepository) GetMealsWithRecipes(
	ctx context.Context,
	planID uuid.UUID,
) ([]models.PlanMeal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pm.id, pm.plan_id, pm.meal_date, pm.meal_slot, pm.recipe_id, pm.servings,
		       r.id, r.user_id, r.name, r.description, r.base_servings, r.is_shared
		FROM recipes.plan_meals pm
		JOIN recipes.recipes r ON r.id = pm.recipe_id
		WHERE pm.plan_id = $1
		ORDER BY pm.meal_date, pm.meal_slot`,
		planID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.PlanMeal
	for rows.Next() {
		var meal models.PlanMeal
		var recipe models.Recipe
		if err = rows.Scan(
			&meal.ID,
			&meal.PlanID,
			&meal.MealDate,
			&meal.MealSlot,
			&meal.RecipeID,
			&meal.Servings,
			&recipe.ID, &recipe.UserID, &recipe.Name, &recipe.Description,
			&recipe.BaseServings, &recipe.IsShared,
		); err != nil {
			return nil, err
		}
		meal.Recipe = &recipe
		result = append(result, meal)
	}
	return result, rows.Err()
}

func (r *PlansRepository) SharePlan(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	canEdit bool,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO recipes.plan_access (plan_id, user_id, can_edit)
		VALUES ($1, $2, $3)
		ON CONFLICT (plan_id, user_id) DO UPDATE SET can_edit = EXCLUDED.can_edit`,
		planID, userID, canEdit,
	)
	return err
}
