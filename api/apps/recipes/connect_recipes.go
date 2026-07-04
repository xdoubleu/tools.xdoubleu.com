package recipes

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"github.com/xdoubleu/essentia/v4/pkg/database"

	"tools.xdoubleu.com/apps/recipes/internal/models"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/gen/recipes/v1/recipesv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

type recipesConnectHandler struct {
	app *Recipes
}

var _ recipesv1connect.RecipesServiceHandler = (*recipesConnectHandler)(nil)

func getUser(ctx context.Context) *sharedmodels.User {
	return contexttools.GetValue[sharedmodels.User](
		ctx,
		constants.UserContextKey,
	)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, database.ErrResourceNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	if errors.Is(err, database.ErrResourceConflict) {
		return connect.NewError(connect.CodeAlreadyExists, err)
	}
	// For iapp.HTTPError, map the status code.
	var httpErr *iapp.HTTPError
	if err != nil && func() *iapp.HTTPError {
		target := &iapp.HTTPError{} //nolint:exhaustruct // used for type assertion only
		_ = errors.As(err, &target)
		return target
	}() != nil {
		httpErr = func() *iapp.HTTPError {
			target := &iapp.HTTPError{} //nolint:exhaustruct // used for type assertion only
			_ = errors.As(err, &target)
			return target
		}()

		switch httpErr.Status {
		case http.StatusBadRequest:
			return connect.NewError(connect.CodeInvalidArgument, err)
		case http.StatusNotFound:
			return connect.NewError(connect.CodeNotFound, err)
		case http.StatusConflict:
			return connect.NewError(connect.CodeAlreadyExists, err)
		default:
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewError(connect.CodeInternal, err)
}

func protoRecipe(r *models.Recipe) *recipesv1.Recipe {
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
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     r.UpdatedAt.Format(time.RFC3339),
		Ingredients:   ingredients,
	}
}

func protoRecipes(list []models.Recipe) []*recipesv1.Recipe {
	result := make([]*recipesv1.Recipe, len(list))
	for i := range list {
		result[i] = protoRecipe(&list[i])
	}
	return result
}

func protoIngredient(ing *models.Ingredient) *recipesv1.Ingredient {
	if ing == nil {
		return nil
	}
	return &recipesv1.Ingredient{
		Id:        ing.ID.String(),
		RecipeId:  ing.RecipeID.String(),
		Name:      ing.Name,
		Amount:    ing.Amount,
		Unit:      ing.Unit,
		SortOrder: int32(ing.SortOrder), //nolint:gosec // int32 safe for domain values
		GroupName: ing.GroupName,
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

// dtoToRecipe converts request fields to domain models.
func dtoToRecipe(
	name string,
	steps []string,
	baseServings int32,
	ingredientNames []string,
	ingredientAmounts []float64,
	ingredientUnits []string,
	ingredientGroupNames []string,
) (models.Recipe, []models.Ingredient) {
	var nonEmpty []string
	for _, s := range steps {
		if t := strings.TrimSpace(s); t != "" {
			nonEmpty = append(nonEmpty, t)
		}
	}

	servings := int(baseServings)
	if servings <= 0 {
		servings = 2
	}

	//nolint:exhaustruct // other fields optional
	recipe := models.Recipe{
		Name:         name,
		Instructions: strings.Join(nonEmpty, "\n"),
		BaseServings: servings,
	}

	var ingredients []models.Ingredient
	for i := range ingredientNames {
		ingredientName := ingredientNames[i]
		if ingredientName == "" {
			continue
		}
		var amount float64
		if i < len(ingredientAmounts) {
			amount = ingredientAmounts[i]
		}
		unit := ""
		if i < len(ingredientUnits) {
			unit = strings.TrimSpace(ingredientUnits[i])
		}
		var groupName *string
		if i < len(ingredientGroupNames) && ingredientGroupNames[i] != "" {
			g := strings.TrimSpace(ingredientGroupNames[i])
			if g != "" {
				groupName = &g
			}
		}
		//nolint:exhaustruct // other fields optional
		ingredients = append(ingredients, models.Ingredient{
			Name:      ingredientName,
			Amount:    amount,
			Unit:      unit,
			SortOrder: i,
			GroupName: groupName,
		})
	}
	return recipe, ingredients
}
