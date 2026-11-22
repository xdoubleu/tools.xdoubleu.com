package connect

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

// Handler serves the RecipesService RPCs.
type Handler struct {
	recipes *services.RecipeService
}

// NewHandler returns a Handler backed by svc.
func NewHandler(svc *services.RecipeService) *Handler {
	return &Handler{recipes: svc}
}

var _ recipesv1connect.RecipesServiceHandler = (*Handler)(nil)

// CallerID returns the authenticated caller's user ID.
func CallerID(ctx context.Context) (string, error) {
	user := getUser(ctx)
	if user == nil {
		return "", connect.Errorf(connect.CodeUnauthenticated, "user not authenticated")
	}
	return user.ID, nil
}

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		ctx,
		constants.UserContextKey,
	)
}

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

// ProtoRecipe maps a recipe to its wire form.
func ProtoRecipe(r *models.Recipe) *recipesv1.Recipe {
	if r == nil {
		return nil
	}
	ingredients := make([]*recipesv1.Ingredient, len(r.Ingredients))
	for i, ing := range r.Ingredients {
		ingredients[i] = protoIngredient(&ing)
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

func protoRecipes(list []models.Recipe) []*recipesv1.Recipe {
	result := make([]*recipesv1.Recipe, len(list))
	for i := range list {
		result[i] = ProtoRecipe(&list[i])
	}
	return result
}

func protoIngredient(ing *models.Ingredient) *recipesv1.Ingredient {
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

func protoScaledIngredient(
	name string,
	amount float64,
	unit string,
) *recipesv1.ScaledIngredient {
	return &recipesv1.ScaledIngredient{
		Name:   name,
		Amount: toFraction(amount),
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
		scaled[i] = protoScaledIngredient(ing.Name, ing.Amount, ing.Unit)
	}
	return &recipesv1.GetRecipeResponse{
		Recipe: ProtoRecipe(s.Recipe),
		Servings: int32( //nolint:gosec // int32 safe for domain values
			s.Servings,
		),
		IsOwner:           s.Recipe.UserID == userID,
		ScaledIngredients: scaled,
		CanEdit:           s.CanEdit,
	}
}

// recipeInput maps request fields, parallel per ingredient, to a service
// input. Normalisation is the service's job.
func recipeInput(
	id uuid.UUID,
	name string,
	steps []string,
	baseServings int32,
	batchServings *int32,
	isDraft bool,
	mealType, effort string,
	names []string,
	amounts []float64,
	units, groupNames, alternativeGroups []string,
) services.RecipeInput {
	ingredients := make([]services.IngredientInput, len(names))
	for i, name := range names {
		ingredients[i] = services.IngredientInput{
			Name:             name,
			Amount:           at(amounts, i),
			Unit:             at(units, i),
			GroupName:        at(groupNames, i),
			AlternativeGroup: at(alternativeGroups, i),
		}
	}
	var batch *int
	if batchServings != nil {
		v := int(*batchServings)
		batch = &v
	}
	return services.RecipeInput{
		ID:            id,
		Name:          name,
		Steps:         steps,
		BaseServings:  int(baseServings),
		BatchServings: batch,
		IsDraft:       isDraft,
		MealType:      mealType,
		Effort:        effort,
		Ingredients:   ingredients,
	}
}

// at returns values[i], or the zero value when absent.
func at[T any](values []T, i int) T {
	var zero T
	if i >= len(values) {
		return zero
	}
	return values[i]
}
