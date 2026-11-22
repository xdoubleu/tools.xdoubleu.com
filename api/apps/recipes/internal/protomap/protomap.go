// Package protomap maps the recipes domain models and errors to their Connect
// forms, for the Connect and MCP adapters.
package protomap

import (
	"time"

	"connectrpc.com/connect/v2"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

const msgRecipeOnMealPlan = "This recipe is used in a meal plan; remove it there first."

// MapError maps a service error to a Connect error.
func MapError(err error) error {
	return connecttools.MapError(
		err,
		connecttools.CodeWithMessage(
			services.ErrRecipeOnMealPlan,
			connect.CodeFailedPrecondition,
			msgRecipeOnMealPlan,
		),
		connecttools.Code(services.ErrVideoInstagram, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrVideoUnsupportedURL, connect.CodeInvalidArgument),
		connecttools.Code(services.ErrVideoNotFound, connect.CodeNotFound),
		connecttools.Code(services.ErrVideoFetch, connect.CodeUnavailable),
	)
}

// Recipe maps a recipe to its wire form.
func Recipe(r *models.Recipe) *recipesv1.Recipe {
	if r == nil {
		return nil
	}
	ingredients := make([]*recipesv1.Ingredient, len(r.Ingredients))
	for i, ing := range r.Ingredients {
		ingredients[i] = ingredient(&ing)
	}
	var batchServings *int32
	if r.BatchServings != nil {
		v := int32(*r.BatchServings) //nolint:gosec // int32 safe for domain values
		batchServings = &v
	}
	return &recipesv1.Recipe{
		Id:           r.ID.String(),
		UserId:       r.UserID,
		Name:         r.Name,
		Instructions: r.Instructions,
		BaseServings: int32( //nolint:gosec // int32 safe for domain values
			r.BaseServings,
		),
		BatchServings: batchServings,
		IsDraft:       r.IsDraft,
		MealType:      r.MealType,
		Effort:        r.Effort,
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.Format(time.RFC3339),
		Ingredients:   ingredients,
	}
}

// Recipes maps a list of recipes.
func Recipes(list []models.Recipe) []*recipesv1.Recipe {
	result := make([]*recipesv1.Recipe, len(list))
	for i := range list {
		result[i] = Recipe(&list[i])
	}
	return result
}

func ingredient(ing *models.Ingredient) *recipesv1.Ingredient {
	if ing == nil {
		return nil
	}
	return &recipesv1.Ingredient{
		Id:               ing.ID.String(),
		RecipeId:         ing.RecipeID.String(),
		Name:             ing.Name,
		Amount:           ing.Amount,
		Unit:             ing.Unit,
		SortOrder:        int32(ing.SortOrder), //nolint:gosec // int32 safe for domain values
		GroupName:        ing.GroupName,
		AlternativeGroup: ing.AlternativeGroup,
	}
}

func scaledIngredient(
	name string,
	amount float64,
	unit string,
) *recipesv1.ScaledIngredient {
	return &recipesv1.ScaledIngredient{
		Name:   name,
		Amount: ToFraction(amount),
		Unit:   unit,
	}
}

// ScaledRecipeResponse maps a scaled recipe to the GetRecipe response for the
// caller.
func ScaledRecipeResponse(
	userID string,
	s *services.ScaledRecipe,
) *recipesv1.GetRecipeResponse {
	scaled := make([]*recipesv1.ScaledIngredient, len(s.Ingredients))
	for i, ing := range s.Ingredients {
		scaled[i] = scaledIngredient(ing.Name, ing.Amount, ing.Unit)
	}
	return &recipesv1.GetRecipeResponse{
		Recipe: Recipe(s.Recipe),
		Servings: int32( //nolint:gosec // int32 safe for domain values
			s.Servings,
		),
		IsOwner:           s.Recipe.UserID == userID,
		ScaledIngredients: scaled,
		CanEdit:           s.CanEdit,
	}
}
