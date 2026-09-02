package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/internal/database/postgres"
	"tools.xdoubleu.com/internal/pagination"
)

type RecipesRepository struct {
	db postgres.DB
}

func (r *RecipesRepository) ListForFamily(
	ctx context.Context,
	familyID uuid.UUID,
	limit int32,
	offset int32,
) ([]models.Recipe, bool, error) {
	safeLimit, sqlLimit := pagination.Clamp(limit)

	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.user_id, r.family_id, r.name,
		       r.instructions, r.base_servings, r.created_at, r.updated_at
		FROM recipes.recipes r
		WHERE r.family_id = $1
		ORDER BY r.name, r.id
		LIMIT $2 OFFSET $3`,
		familyID, sqlLimit, offset,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var result []models.Recipe
	for rows.Next() {
		var recipe models.Recipe
		if err = rows.Scan(
			&recipe.ID, &recipe.UserID, &recipe.FamilyID, &recipe.Name,
			&recipe.Instructions, &recipe.BaseServings,
			&recipe.CreatedAt, &recipe.UpdatedAt,
		); err != nil {
			return nil, false, err
		}
		result = append(result, recipe)
	}
	if err = rows.Err(); err != nil {
		return nil, false, err
	}

	page, hasMore := pagination.Split(result, safeLimit)
	return page, hasMore, nil
}

func (r *RecipesRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*models.Recipe, error) {
	var recipe models.Recipe
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, family_id, name,
		instructions, base_servings, batch_servings, created_at, updated_at
		FROM recipes.recipes
		WHERE id = $1`,
		id,
	).Scan(
		&recipe.ID, &recipe.UserID, &recipe.FamilyID, &recipe.Name,
		&recipe.Instructions, &recipe.BaseServings, &recipe.BatchServings,
		&recipe.CreatedAt, &recipe.UpdatedAt,
	)
	if err != nil {
		return nil, postgres.PgxErrorToHTTPError(err)
	}
	return &recipe, nil
}

func (r *RecipesRepository) Create(
	ctx context.Context,
	recipe models.Recipe,
) (*models.Recipe, error) {
	err := r.db.QueryRow(
		ctx,
		`INSERT INTO recipes.recipes
		(user_id, family_id, name, instructions, base_servings, batch_servings)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		recipe.UserID,
		recipe.FamilyID,
		recipe.Name,
		recipe.Instructions,
		recipe.BaseServings,
		recipe.BatchServings,
	).Scan(&recipe.ID, &recipe.CreatedAt, &recipe.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &recipe, nil
}

func (r *RecipesRepository) Update(
	ctx context.Context,
	recipe models.Recipe,
) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE recipes.recipes
		SET name = $3, instructions = $4,
		base_servings = $5, batch_servings = $6, updated_at = now()
		WHERE id = $1 AND family_id = $2`,
		recipe.ID,
		recipe.FamilyID,
		recipe.Name,
		recipe.Instructions,
		recipe.BaseServings,
		recipe.BatchServings,
	)
	return err
}

func (r *RecipesRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	familyID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.recipes WHERE id = $1 AND family_id = $2`,
		id, familyID,
	)
	return err
}

func (r *RecipesRepository) ReplaceIngredients(
	ctx context.Context,
	recipeID uuid.UUID,
	ingredients []models.Ingredient,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.ingredients WHERE recipe_id = $1`,
		recipeID,
	)
	if err != nil {
		return err
	}

	if len(ingredients) == 0 {
		return nil
	}

	//nolint:exhaustruct //other fields optional
	batch := &pgx.Batch{}
	for i, ing := range ingredients {
		batch.Queue(`
			INSERT INTO recipes.ingredients
			(recipe_id, name, amount, unit, sort_order, group_name)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			recipeID, ing.Name, ing.Amount, ing.Unit, i, ing.GroupName,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	for range ingredients {
		if _, err = br.Exec(); err != nil {
			_ = br.Close()
			return err
		}
	}
	return br.Close()
}

func (r *RecipesRepository) GetIngredients(
	ctx context.Context,
	recipeID uuid.UUID,
) ([]models.Ingredient, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, recipe_id, name, amount, unit, sort_order, group_name
		FROM recipes.ingredients
		WHERE recipe_id = $1
		ORDER BY sort_order`,
		recipeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Ingredient
	for rows.Next() {
		var ing models.Ingredient
		if err = rows.Scan(
			&ing.ID, &ing.RecipeID, &ing.Name, &ing.Amount,
			&ing.Unit, &ing.SortOrder, &ing.GroupName,
		); err != nil {
			return nil, err
		}
		result = append(result, ing)
	}
	return result, rows.Err()
}
