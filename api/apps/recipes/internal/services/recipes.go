package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/internal/app"
)

const errNotInFamily = "You do not have access to this recipe"

// recipesStore is the storage surface RecipeService needs. It is satisfied by
// repositories.RecipesRepository and by fakes in unit tests, so the
// family-scoping rules can be tested without a database.
type recipesStore interface {
	ListForFamily(
		ctx context.Context, familyID uuid.UUID, limit, offset int32,
	) ([]models.Recipe, bool, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Recipe, error)
	GetIngredients(ctx context.Context, id uuid.UUID) ([]models.Ingredient, error)
	Create(ctx context.Context, recipe models.Recipe) (*models.Recipe, error)
	ReplaceIngredients(
		ctx context.Context,
		id uuid.UUID,
		ingredients []models.Ingredient,
	) error
	Update(ctx context.Context, recipe models.Recipe) error
	Delete(ctx context.Context, id uuid.UUID, familyID uuid.UUID) error
}

// familyStore resolves which family a user belongs to, lazily creating a
// family-of-one the first time it's asked for (see internal/family).
type familyStore interface {
	EnsureFamily(ctx context.Context, userID string) (uuid.UUID, error)
}

type RecipeService struct {
	repo   recipesStore
	family familyStore
}

func (s *RecipeService) List(
	ctx context.Context,
	userID string,
	limit int32,
	offset int32,
) ([]models.Recipe, bool, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	return s.repo.ListForFamily(ctx, familyID, limit, offset)
}

// Get returns a recipe belonging to the user's family. Every family member
// has equal read/write access, so canEdit is always true once access is
// granted at all.
func (s *RecipeService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Recipe, bool, error) {
	recipe, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, false, err
	}

	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	if recipe.FamilyID != familyID {
		return nil, false, &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: errNotInFamily,
		}
	}

	ingredients, err := s.repo.GetIngredients(ctx, id)
	if err != nil {
		return nil, false, err
	}
	recipe.Ingredients = ingredients
	return recipe, true, nil
}

// GetScaled returns a recipe belonging to the user's family, along with the
// resolved serving count and its ingredients scaled to that count.
// requestedServings <= 0 keeps the recipe's own BaseServings.
func (s *RecipeService) GetScaled(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	requestedServings int,
) (*models.Recipe, bool, int, []models.Ingredient, error) {
	recipe, canEdit, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, false, 0, nil, err
	}

	servings := recipe.BaseServings
	if requestedServings > 0 {
		servings = requestedServings
	}

	ratio := float64(servings) / float64(recipe.BaseServings)
	scaled := make([]models.Ingredient, len(recipe.Ingredients))
	for i, ing := range recipe.Ingredients {
		scaled[i] = ing
		scaled[i].Amount = ing.Amount * ratio
	}

	return recipe, canEdit, servings, scaled, nil
}

func (s *RecipeService) Create(
	ctx context.Context,
	userID string,
	recipe models.Recipe,
) (*models.Recipe, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	recipe.UserID = userID
	recipe.FamilyID = familyID

	created, err := s.repo.Create(ctx, recipe)
	if err != nil {
		return nil, err
	}

	if err = s.repo.ReplaceIngredients(ctx, created.ID, recipe.Ingredients); err != nil {
		return nil, err
	}
	created.Ingredients = recipe.Ingredients
	return created, nil
}

func (s *RecipeService) Update(
	ctx context.Context,
	userID string,
	recipe models.Recipe,
) error {
	existing, err := s.repo.GetByID(ctx, recipe.ID)
	if err != nil {
		return err
	}

	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	if existing.FamilyID != familyID {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: errNotInFamily,
		}
	}

	// Recipes always remain attributed to their original creator, even when
	// another family member updates them.
	recipe.UserID = existing.UserID
	recipe.FamilyID = existing.FamilyID
	if err = s.repo.Update(ctx, recipe); err != nil {
		return err
	}
	return s.repo.ReplaceIngredients(ctx, recipe.ID, recipe.Ingredients)
}

func (s *RecipeService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	if existing.FamilyID != familyID {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: errNotInFamily,
		}
	}
	return s.repo.Delete(ctx, id, familyID)
}
