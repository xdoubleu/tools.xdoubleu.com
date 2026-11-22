package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/recipes/internal/services"
	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
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
