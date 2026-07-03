package recipes

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
)

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

	recipe, canEdit, err := h.app.services.Recipes.Get(ctx, id, user.ID)
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
		CanEdit:           canEdit,
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
		req.Msg.IngredientGroupNames,
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
		req.Msg.IngredientGroupNames,
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
