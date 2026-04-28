package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/repositories"
)

type RecipeService struct {
	repo *repositories.RecipesRepository
}

func (s *RecipeService) List(
	ctx context.Context,
	userID string,
) ([]models.Recipe, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *RecipeService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Recipe, error) {
	recipe, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	sharedWith, err := s.repo.GetSharedUserIDs(ctx, id)
	if err != nil {
		return nil, err
	}

	isSharedWithUser := false
	for _, uid := range sharedWith {
		if uid == userID {
			isSharedWithUser = true
			break
		}
	}

	if recipe.UserID != userID && !isSharedWithUser {
		return nil, &HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not have access to this recipe",
		}
	}

	ingredients, err := s.repo.GetIngredients(ctx, id)
	if err != nil {
		return nil, err
	}
	recipe.Ingredients = ingredients
	recipe.SharedWith = sharedWith
	return recipe, nil
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
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not own this recipe",
		}
	}

	recipe.UserID = userID
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
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not own this recipe",
		}
	}
	return s.repo.Delete(ctx, id, userID)
}

func (s *RecipeService) Share(
	ctx context.Context,
	recipeID uuid.UUID,
	ownerID, targetUserID string,
) error {
	existing, err := s.repo.GetByID(ctx, recipeID)
	if err != nil {
		return err
	}
	if existing.UserID != ownerID {
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not own this recipe",
		}
	}
	return s.repo.ShareRecipe(ctx, recipeID, targetUserID)
}

func (s *RecipeService) Unshare(
	ctx context.Context,
	recipeID uuid.UUID,
	ownerID, targetUserID string,
) error {
	existing, err := s.repo.GetByID(ctx, recipeID)
	if err != nil {
		return err
	}
	if existing.UserID != ownerID {
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not own this recipe",
		}
	}
	return s.repo.UnshareRecipe(ctx, recipeID, targetUserID)
}
