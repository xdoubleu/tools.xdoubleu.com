package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/xdoubleu/essentia/v4/pkg/database/postgres"
	"tools.xdoubleu.com/apps/recipes/internal/models"
)

type RecipesRepository struct {
	db postgres.DB
}

func (r *RecipesRepository) ListForUser(
	ctx context.Context,
	userID string,
) ([]models.Recipe, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT r.id, r.user_id, r.name,
		       r.instructions, r.base_servings, r.created_at, r.updated_at
		FROM recipes.recipes r
		LEFT JOIN recipes.recipe_access ra ON ra.recipe_id = r.id AND ra.user_id = $1
		WHERE r.user_id = $1 OR ra.user_id = $1
		ORDER BY r.name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Recipe
	for rows.Next() {
		var recipe models.Recipe
		if err = rows.Scan(
			&recipe.ID, &recipe.UserID, &recipe.Name,
			&recipe.Instructions, &recipe.BaseServings,
			&recipe.CreatedAt, &recipe.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, recipe)
	}
	return result, rows.Err()
}

func (r *RecipesRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*models.Recipe, error) {
	var recipe models.Recipe
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, name,
		instructions, base_servings, created_at, updated_at
		FROM recipes.recipes
		WHERE id = $1`,
		id,
	).Scan(
		&recipe.ID, &recipe.UserID, &recipe.Name,
		&recipe.Instructions, &recipe.BaseServings, &recipe.CreatedAt, &recipe.UpdatedAt,
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
		(user_id, name, instructions, base_servings)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`,
		recipe.UserID,
		recipe.Name,
		recipe.Instructions,
		recipe.BaseServings,
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
		base_servings = $5, updated_at = now()
		WHERE id = $1 AND user_id = $2`,
		recipe.ID,
		recipe.UserID,
		recipe.Name,
		recipe.Instructions,
		recipe.BaseServings,
	)
	return err
}

func (r *RecipesRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.recipes WHERE id = $1 AND user_id = $2`,
		id, userID,
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
			INSERT INTO recipes.ingredients (recipe_id, name, amount, unit, sort_order)
			VALUES ($1, $2, $3, $4, $5)`,
			recipeID, ing.Name, ing.Amount, ing.Unit, i,
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
		SELECT id, recipe_id, name, amount, unit, sort_order
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
			&ing.ID, &ing.RecipeID, &ing.Name, &ing.Amount, &ing.Unit, &ing.SortOrder,
		); err != nil {
			return nil, err
		}
		result = append(result, ing)
	}
	return result, rows.Err()
}

func (r *RecipesRepository) ShareRecipe(
	ctx context.Context,
	recipeID uuid.UUID,
	targetUserID string,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO recipes.recipe_access (recipe_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`,
		recipeID, targetUserID,
	)
	return err
}

func (r *RecipesRepository) UnshareRecipe(
	ctx context.Context,
	recipeID uuid.UUID,
	targetUserID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.recipe_access WHERE recipe_id = $1 AND user_id = $2`,
		recipeID, targetUserID,
	)
	return err
}

func (r *RecipesRepository) GetSharedUserIDs(
	ctx context.Context,
	recipeID uuid.UUID,
) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT user_id FROM recipes.recipe_access WHERE recipe_id = $1`,
		recipeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var uid string
		if err = rows.Scan(&uid); err != nil {
			return nil, err
		}
		result = append(result, uid)
	}
	return result, rows.Err()
}
