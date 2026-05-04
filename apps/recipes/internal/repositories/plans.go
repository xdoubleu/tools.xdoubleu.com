package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
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
		SELECT p.id, p.owner_user_id, p.name,
		       p.ical_token, p.created_at, p.updated_at,
		       COALESCE(pa.can_edit, p.owner_user_id = $1) AS can_edit,
		       p.ical_hide_slots, p.ical_hide_past
		FROM recipes.plans p
		LEFT JOIN recipes.plan_access pa ON pa.plan_id = p.id AND pa.user_id = $1
		WHERE p.owner_user_id = $1 OR pa.user_id = $1
		ORDER BY p.name`,
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
			&plan.ID, &plan.OwnerUserID, &plan.Name,
			&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt, &plan.CanEdit,
			&plan.ICalHideSlots, &plan.ICalHidePast,
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
		SELECT p.id, p.owner_user_id, p.name,
		       p.ical_token, p.created_at, p.updated_at,
		       COALESCE(pa.can_edit, p.owner_user_id = $2) AS can_edit,
		       p.ical_hide_slots, p.ical_hide_past
		FROM recipes.plans p
		LEFT JOIN recipes.plan_access pa ON pa.plan_id = p.id AND pa.user_id = $2
		WHERE p.id = $1 AND (p.owner_user_id = $2 OR pa.user_id = $2)`,
		id, userID,
	).Scan(
		&plan.ID, &plan.OwnerUserID, &plan.Name,
		&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt, &plan.CanEdit,
		&plan.ICalHideSlots, &plan.ICalHidePast,
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
		SELECT id, owner_user_id, name,
		       ical_token, created_at, updated_at,
		       ical_hide_slots, ical_hide_past
		FROM recipes.plans
		WHERE ical_token = $1`,
		token,
	).Scan(
		&plan.ID, &plan.OwnerUserID, &plan.Name,
		&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt,
		&plan.ICalHideSlots, &plan.ICalHidePast,
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
		INSERT INTO recipes.plans (owner_user_id, name)
		VALUES ($1, $2)
		RETURNING id, ical_token, created_at, updated_at`,
		plan.OwnerUserID, plan.Name,
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
		SET name = $3, ical_hide_slots = $4, ical_hide_past = $5,
		    updated_at = now()
		WHERE id = $1 AND owner_user_id = $2`,
		plan.ID, plan.OwnerUserID, plan.Name,
		plan.ICalHideSlots, plan.ICalHidePast,
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
		INSERT INTO recipes.plan_meals
		       (plan_id, meal_date, meal_slot, recipe_id, custom_name, servings)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (plan_id, meal_date, meal_slot)
		DO UPDATE SET recipe_id   = EXCLUDED.recipe_id,
		              custom_name = EXCLUDED.custom_name,
		              servings    = EXCLUDED.servings
		RETURNING id`,
		meal.PlanID, meal.MealDate, meal.MealSlot,
		meal.RecipeID, meal.CustomName, meal.Servings,
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

// GetMealsInWindow returns meals for a plan within the given date range.
// When start is zero, all meals are returned (used for iCal export).
func (r *PlansRepository) GetMealsInWindow(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
) ([]models.PlanMeal, error) {
	var rows pgx.Rows
	var err error

	const baseCols = `
		SELECT pm.id, pm.plan_id, pm.meal_date, pm.meal_slot,
		       pm.recipe_id, pm.custom_name, pm.servings,
		       r.id, r.user_id, r.name, r.instructions, r.base_servings
		FROM recipes.plan_meals pm
		LEFT JOIN recipes.recipes r ON r.id = pm.recipe_id`

	if start.IsZero() {
		rows, err = r.db.Query(ctx,
			baseCols+`
			WHERE pm.plan_id = $1
			ORDER BY pm.meal_date, pm.meal_slot`,
			planID,
		)
	} else {
		rows, err = r.db.Query(ctx,
			baseCols+`
			WHERE pm.plan_id = $1
			  AND pm.meal_date BETWEEN $2 AND $3
			ORDER BY pm.meal_date, pm.meal_slot`,
			planID, start, end,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.PlanMeal
	for rows.Next() {
		var meal models.PlanMeal
		var rID *uuid.UUID
		var rUserID, rName, rInstructions *string
		var rBaseServings *int
		if err = rows.Scan(
			&meal.ID,
			&meal.PlanID,
			&meal.MealDate,
			&meal.MealSlot,
			&meal.RecipeID,
			&meal.CustomName,
			&meal.Servings,
			&rID, &rUserID, &rName, &rInstructions, &rBaseServings,
		); err != nil {
			return nil, err
		}
		if rID != nil {
			//nolint:exhaustruct // other fields not needed here
			meal.Recipe = &models.Recipe{
				ID:           *rID,
				UserID:       *rUserID,
				Name:         *rName,
				Instructions: *rInstructions,
				BaseServings: *rBaseServings,
			}
		}
		result = append(result, meal)
	}
	return result, rows.Err()
}

func (r *PlansRepository) GetSharedWith(
	ctx context.Context,
	planID uuid.UUID,
	ownerID string,
) ([]models.PlanSharedUser, error) {
	rows, err := r.db.Query(ctx, `
		SELECT pa.user_id, pa.can_edit,
		       COALESCE(c.display_name, pa.user_id) AS display_name
		FROM recipes.plan_access pa
		LEFT JOIN global.contacts c
		       ON c.owner_user_id = $2 AND c.contact_user_id = pa.user_id
		WHERE pa.plan_id = $1
		ORDER BY display_name`,
		planID, ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.PlanSharedUser
	for rows.Next() {
		var u models.PlanSharedUser
		if err = rows.Scan(&u.UserID, &u.CanEdit, &u.DisplayName); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

func (r *PlansRepository) UnshareUser(
	ctx context.Context,
	planID uuid.UUID,
	targetUserID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.plan_access WHERE plan_id = $1 AND user_id = $2`,
		planID, targetUserID,
	)
	return err
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
