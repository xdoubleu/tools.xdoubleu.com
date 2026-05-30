package recipes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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

// ── Shared Helpers ────────────────────────────────────────────────────────

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

// ── Proto conversion helpers ───────────────────────────────────────────────

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
		SharedWith:    r.SharedWith,
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
		//nolint:exhaustruct // other fields optional
		ingredients = append(ingredients, models.Ingredient{
			Name:      ingredientName,
			Amount:    amount,
			Unit:      unit,
			SortOrder: i,
		})
	}
	return recipe, ingredients
}

// ── Recipe RPCs ────────────────────────────────────────────────────────────

func (h *recipesConnectHandler) ListRecipes(
	ctx context.Context,
	_ *connect.Request[recipesv1.ListRecipesRequest],
) (*connect.Response[recipesv1.ListRecipesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	list, err := h.app.services.Recipes.List(ctx, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.ListRecipesResponse{
		Recipes: protoRecipes(list),
	}), nil
}

func (h *recipesConnectHandler) GetRecipe(
	ctx context.Context,
	req *connect.Request[recipesv1.GetRecipeRequest],
) (*connect.Response[recipesv1.GetRecipeResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid recipe ID"),
		)
	}

	recipe, err := h.app.services.Recipes.Get(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	servings := recipe.BaseServings
	if req.Msg.Servings > 0 {
		servings = int(req.Msg.Servings)
	}

	scaled := make([]*recipesv1.ScaledIngredient, len(recipe.Ingredients))
	for i, ing := range recipe.Ingredients {
		ratio := float64(servings) / float64(recipe.BaseServings)
		scaled[i] = protoScaledIngredient(ing.Name, ing.Amount*ratio, ing.Unit)
	}

	return connect.NewResponse(&recipesv1.GetRecipeResponse{
		Recipe: protoRecipe(recipe),
		Servings: int32( //nolint:gosec // int32 safe for domain values
			servings,
		),
		IsOwner:           recipe.UserID == user.ID,
		ScaledIngredients: scaled,
	}), nil
}

func (h *recipesConnectHandler) CreateRecipe(
	ctx context.Context,
	req *connect.Request[recipesv1.CreateRecipeRequest],
) (*connect.Response[recipesv1.CreateRecipeResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	recipe, ingredients := dtoToRecipe(
		req.Msg.Name,
		req.Msg.Steps,
		req.Msg.BaseServings,
		req.Msg.IngredientNames,
		req.Msg.IngredientAmounts,
		req.Msg.IngredientUnits,
	)
	recipe.Ingredients = ingredients
	if req.Msg.BatchServings != nil {
		v := int(*req.Msg.BatchServings)
		recipe.BatchServings = &v
	}

	created, err := h.app.services.Recipes.Create(ctx, user.ID, recipe)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.CreateRecipeResponse{
		Recipe: protoRecipe(created),
	}), nil
}

func (h *recipesConnectHandler) UpdateRecipe(
	ctx context.Context,
	req *connect.Request[recipesv1.UpdateRecipeRequest],
) (*connect.Response[recipesv1.UpdateRecipeResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid recipe ID"),
		)
	}

	recipe, ingredients := dtoToRecipe(
		req.Msg.Name,
		req.Msg.Steps,
		req.Msg.BaseServings,
		req.Msg.IngredientNames,
		req.Msg.IngredientAmounts,
		req.Msg.IngredientUnits,
	)
	recipe.ID = id
	recipe.Ingredients = ingredients
	if req.Msg.BatchServings != nil {
		v := int(*req.Msg.BatchServings)
		recipe.BatchServings = &v
	}

	err = h.app.services.Recipes.Update(ctx, user.ID, recipe)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.UpdateRecipeResponse{}), nil
}

func (h *recipesConnectHandler) DeleteRecipe(
	ctx context.Context,
	req *connect.Request[recipesv1.DeleteRecipeRequest],
) (*connect.Response[recipesv1.DeleteRecipeResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid recipe ID"),
		)
	}

	err = h.app.services.Recipes.Delete(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.DeleteRecipeResponse{}), nil
}

func (h *recipesConnectHandler) ShareRecipe(
	ctx context.Context,
	req *connect.Request[recipesv1.ShareRecipeRequest],
) (*connect.Response[recipesv1.ShareRecipeResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid recipe ID"),
		)
	}

	err = h.app.services.Recipes.Share(ctx, id, user.ID, req.Msg.ContactUserId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.ShareRecipeResponse{}), nil
}

func (h *recipesConnectHandler) UnshareRecipe(
	ctx context.Context,
	req *connect.Request[recipesv1.UnshareRecipeRequest],
) (*connect.Response[recipesv1.UnshareRecipeResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	id, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid recipe ID"),
		)
	}

	if req.Msg.TargetUserId == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("target user ID is required"),
		)
	}

	err = h.app.services.Recipes.Unshare(ctx, id, user.ID, req.Msg.TargetUserId)
	if err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&recipesv1.UnshareRecipeResponse{}), nil
}
