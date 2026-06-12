package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/repositories"
	"tools.xdoubleu.com/internal/app"
)

const errNotRecipeOwner = "You do not own this recipe"

type RecipeService struct {
	repo *repositories.RecipesRepository
}

func (s *RecipeService) List(
	ctx context.Context,
	userID string,
) ([]models.Recipe, error) {
	return s.repo.ListForUser(ctx, userID)
}

// Get returns a recipe the user owns or has book access to, along with whether
// the user may edit it.
func (s *RecipeService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Recipe, bool, error) {
	recipe, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, false, err
	}

	canEdit := recipe.UserID == userID
	if recipe.UserID != userID {
		shareEdit, ok, accessErr := s.repo.GetBookAccess(ctx, recipe.UserID, userID)
		if accessErr != nil {
			return nil, false, accessErr
		}
		if !ok {
			return nil, false, &app.HTTPError{
				Status:  http.StatusForbidden,
				Message: "You do not have access to this recipe",
			}
		}
		canEdit = shareEdit
	}

	ingredients, err := s.repo.GetIngredients(ctx, id)
	if err != nil {
		return nil, false, err
	}
	recipe.Ingredients = ingredients
	return recipe, canEdit, nil
}

func (s *RecipeService) Create(
	ctx context.Context,
	userID string,
	recipe models.Recipe,
) (*models.Recipe, error) {
	recipe.UserID = userID
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
	if existing.UserID != userID {
		canEdit, ok, accessErr := s.repo.GetBookAccess(ctx, existing.UserID, userID)
		if accessErr != nil {
			return accessErr
		}
		if !ok || !canEdit {
			return &app.HTTPError{
				Status:  http.StatusForbidden,
				Message: errNotRecipeOwner,
			}
		}
	}

	// Recipes always remain owned by their original creator, even when an
	// edit-sharer updates them.
	recipe.UserID = existing.UserID
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
	if existing.UserID != userID {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: errNotRecipeOwner,
		}
	}
	return s.repo.Delete(ctx, id, userID)
}

// ShareBook shares the owner's whole recipe book with targetUserID.
func (s *RecipeService) ShareBook(
	ctx context.Context,
	ownerID, targetUserID string,
	canEdit bool,
) error {
	if targetUserID == "" || targetUserID == ownerID {
		return &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid contact to share with",
		}
	}
	return s.repo.ShareBook(ctx, ownerID, targetUserID, canEdit)
}

func (s *RecipeService) UnshareBook(
	ctx context.Context,
	ownerID, targetUserID string,
) error {
	return s.repo.UnshareBook(ctx, ownerID, targetUserID)
}

func (s *RecipeService) ListBookShares(
	ctx context.Context,
	ownerID string,
) ([]models.RecipeBookShare, error) {
	return s.repo.ListBookShares(ctx, ownerID)
}
