package repositories

import (
	"context"
	"errors"

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
		SELECT r.id, r.user_id, r.name,
		       r.instructions, r.base_servings, r.created_at, r.updated_at
		FROM recipes.recipes r
		WHERE r.user_id = $1
		   OR r.user_id IN (
		       SELECT owner_user_id FROM recipes.recipebook_access
		       WHERE user_id = $1
		   )
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
		instructions, base_servings, batch_servings, created_at, updated_at
		FROM recipes.recipes
		WHERE id = $1`,
		id,
	).Scan(
		&recipe.ID, &recipe.UserID, &recipe.Name,
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
		(user_id, name, instructions, base_servings, batch_servings)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		recipe.UserID,
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
		WHERE id = $1 AND user_id = $2`,
		recipe.ID,
		recipe.UserID,
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

// ShareBook grants targetUserID access to ownerID's whole recipe book.
func (r *RecipesRepository) ShareBook(
	ctx context.Context,
	ownerID, targetUserID string,
	canEdit bool,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO recipes.recipebook_access (owner_user_id, user_id, can_edit)
		VALUES ($1, $2, $3)
		ON CONFLICT (owner_user_id, user_id)
		DO UPDATE SET can_edit = EXCLUDED.can_edit`,
		ownerID, targetUserID, canEdit,
	)
	return err
}

func (r *RecipesRepository) UnshareBook(
	ctx context.Context,
	ownerID, targetUserID string,
) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM recipes.recipebook_access
		WHERE owner_user_id = $1 AND user_id = $2`,
		ownerID, targetUserID,
	)
	return err
}

// ListBookShares returns the users ownerID shares their recipe book with,
// resolving display names from the owner's contacts.
func (r *RecipesRepository) ListBookShares(
	ctx context.Context,
	ownerID string,
) ([]models.RecipeBookShare, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ba.user_id, ba.can_edit,
		       COALESCE(c.display_name, ba.user_id) AS display_name
		FROM recipes.recipebook_access ba
		LEFT JOIN global.contacts c
		       ON c.owner_user_id = $1 AND c.contact_user_id = ba.user_id
		WHERE ba.owner_user_id = $1
		ORDER BY display_name`,
		ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.RecipeBookShare
	for rows.Next() {
		var s models.RecipeBookShare
		if err = rows.Scan(&s.UserID, &s.CanEdit, &s.DisplayName); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetBookAccess reports whether viewerID may access ownerID's book and, if so,
// whether with edit rights.
func (r *RecipesRepository) GetBookAccess(
	ctx context.Context,
	ownerID, viewerID string,
) (bool, bool, error) {
	var canEdit bool
	err := r.db.QueryRow(ctx, `
		SELECT can_edit FROM recipes.recipebook_access
		WHERE owner_user_id = $1 AND user_id = $2`,
		ownerID, viewerID,
	).Scan(&canEdit)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, nil
		}
		return false, false, err
	}
	return canEdit, true, nil
}
