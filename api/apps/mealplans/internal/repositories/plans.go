package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/pagination"
)

type PlansRepository struct {
	db postgres.DB
}

func (r *PlansRepository) ListForFamily(
	ctx context.Context,
	familyID uuid.UUID,
	limit int32,
	offset int32,
) ([]models.Plan, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)

	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.owner_user_id, p.family_id, p.name,
		       p.ical_token, p.created_at, p.updated_at,
		       p.ical_hide_slots, p.ical_hide_past
		FROM mealplans.plans p
		WHERE p.family_id = $1
		ORDER BY p.name, p.id
		LIMIT $2 OFFSET $3`,
		familyID, sqlLimit, offset,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var result []models.Plan
	for rows.Next() {
		var plan models.Plan
		if err = rows.Scan(
			&plan.ID, &plan.OwnerUserID, &plan.FamilyID, &plan.Name,
			&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt,
			&plan.ICalHideSlots, &plan.ICalHidePast,
		); err != nil {
			return nil, false, err
		}
		plan.CanEdit = true
		result = append(result, plan)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}

	page, hasMore := pagination.Split(result, safeLimit)
	return page, hasMore, nil
}

func (r *PlansRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
	familyID uuid.UUID,
) (*models.Plan, error) {
	var plan models.Plan
	err := r.db.QueryRow(ctx, `
		SELECT p.id, p.owner_user_id, p.family_id, p.name,
		       p.ical_token, p.created_at, p.updated_at,
		       p.ical_hide_slots, p.ical_hide_past
		FROM mealplans.plans p
		WHERE p.id = $1 AND p.family_id = $2`,
		id, familyID,
	).Scan(
		&plan.ID, &plan.OwnerUserID, &plan.FamilyID, &plan.Name,
		&plan.ICalToken, &plan.CreatedAt, &plan.UpdatedAt,
		&plan.ICalHideSlots, &plan.ICalHidePast,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	plan.CanEdit = true
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
		FROM mealplans.plans
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
		INSERT INTO mealplans.plans (owner_user_id, family_id, name)
		VALUES ($1, $2, $3)
		RETURNING id, ical_token, created_at, updated_at`,
		plan.OwnerUserID, plan.FamilyID, plan.Name,
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
	// ical_hide_slots is NOT NULL; an omitted repeated field decodes to a nil
	// slice, which pgx would otherwise send as SQL NULL.
	if plan.ICalHideSlots == nil {
		plan.ICalHideSlots = []string{}
	}
	_, err := r.db.Exec(ctx, `
		UPDATE mealplans.plans
		SET name = $3, ical_hide_slots = $4, ical_hide_past = $5,
		    updated_at = now()
		WHERE id = $1 AND family_id = $2`,
		plan.ID, plan.FamilyID, plan.Name,
		plan.ICalHideSlots, plan.ICalHidePast,
	)
	return err
}

func (r *PlansRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	familyID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM mealplans.plans WHERE id = $1 AND family_id = $2`,
		id, familyID,
	)
	return err
}

func (r *PlansRepository) CreateMeal(
	ctx context.Context,
	meal models.PlanMeal,
) (*models.PlanMeal, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO mealplans.plan_meals
		       (plan_id, meal_date, meal_slot, recipe_id, custom_name,
		        servings, exclude_from_shopping_list)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		meal.PlanID, meal.MealDate, meal.MealSlot,
		meal.RecipeID, meal.CustomName, meal.Servings,
		meal.ExcludeFromShoppingList,
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
		`DELETE FROM mealplans.plan_meals WHERE id = $1 AND plan_id = $2`,
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
		       pm.exclude_from_shopping_list, r.name
		FROM mealplans.plan_meals pm
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
		var recipeName *string
		if err = rows.Scan(
			&meal.ID,
			&meal.PlanID,
			&meal.MealDate,
			&meal.MealSlot,
			&meal.RecipeID,
			&meal.CustomName,
			&meal.Servings,
			&meal.ExcludeFromShoppingList,
			&recipeName,
		); err != nil {
			return nil, err
		}
		if recipeName != nil {
			meal.RecipeName = *recipeName
		}
		result = append(result, meal)
	}
	return result, rows.Err()
}

// SuggestRecipes returns recipe IDs previously planned in the same plan on the
// same weekday and meal slot as mealDate, ranked by how often they were used
// (most recent breaking ties). Used to suggest entries when adding a meal.
func (r *PlansRepository) SuggestRecipes(
	ctx context.Context,
	planID uuid.UUID,
	mealDate time.Time,
	slot string,
	limit int,
) ([]models.RecipeSuggestion, error) {
	rows, err := r.db.Query(ctx, `
		SELECT recipe_id, mode() WITHIN GROUP (ORDER BY servings) AS servings
		FROM mealplans.plan_meals
		WHERE plan_id = $1
		  AND recipe_id IS NOT NULL
		  AND meal_slot = $2
		  AND EXTRACT(DOW FROM meal_date) = EXTRACT(DOW FROM $3::date)
		GROUP BY recipe_id
		ORDER BY COUNT(*) DESC, MAX(meal_date) DESC
		LIMIT $4`,
		planID, slot, mealDate, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.RecipeSuggestion
	for rows.Next() {
		var s models.RecipeSuggestion
		if err = rows.Scan(&s.RecipeID, &s.Servings); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *PlansRepository) MoveMeal(
	ctx context.Context,
	mealID uuid.UUID,
	planID uuid.UUID,
	newDate time.Time,
	newSlot string,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE mealplans.plan_meals
		SET meal_date = $3, meal_slot = $4
		WHERE id = $1 AND plan_id = $2`,
		mealID, planID, newDate, newSlot,
	)
	return postgres.PgxErrorToHTTPError(err)
}
