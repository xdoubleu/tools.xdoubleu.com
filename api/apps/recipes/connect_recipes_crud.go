package recipes

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *recipesConnectHandler) ListRecipes(
	ctx context.Context,
	req *recipesv1.ListRecipesRequest,
) (*recipesv1.ListRecipesResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	list, hasMore, err := h.app.services.Recipes.List(
		ctx, user.ID, req.Limit, req.Offset,
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &recipesv1.ListRecipesResponse{
		Recipes: protoRecipes(list),
		HasMore: hasMore,
	}, nil
}

func (h *recipesConnectHandler) GetRecipe(
	ctx context.Context,
	req *recipesv1.GetRecipeRequest,
) (*recipesv1.GetRecipeResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid recipe ID",
		)
	}

	recipe, canEdit, servings, scaledIngredients, err := h.app.services.Recipes.GetScaled(
		ctx,
		id,
		user.ID,
		int(req.Servings),
	)
	if err != nil {
		return nil, mapError(err)
	}

	scaled := make([]*recipesv1.ScaledIngredient, len(scaledIngredients))
	for i, ing := range scaledIngredients {
		scaled[i] = protoScaledIngredient(ing.Name, ing.Amount, ing.Unit)
	}

	return &recipesv1.GetRecipeResponse{
		Recipe: protoRecipe(recipe),
		Servings: int32( //nolint:gosec // int32 safe for domain values
			servings,
		),
		IsOwner:           recipe.UserID == user.ID,
		ScaledIngredients: scaled,
		CanEdit:           canEdit,
	}, nil
}

func (h *recipesConnectHandler) CreateRecipe(
	ctx context.Context,
	req *recipesv1.CreateRecipeRequest,
) (*recipesv1.CreateRecipeResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	recipe, ingredients := dtoToRecipe(
		req.Name,
		req.Steps,
		req.BaseServings,
		req.IngredientNames,
		req.IngredientAmounts,
		req.IngredientUnits,
		req.IngredientGroupNames,
	)
	recipe.Ingredients = ingredients
	recipe.IsDraft = req.IsDraft
	if req.BatchServings != nil {
		v := int(*req.BatchServings)
		recipe.BatchServings = &v
	}
	clientID, err := connecttools.ParseClientID(req.Id)
	if err != nil {
		return nil, err
	}
	recipe.ID = clientID.UUID

	created, err := h.app.services.Recipes.Create(ctx, user.ID, recipe)
	if err != nil {
		return nil, mapError(err)
	}

	return &recipesv1.CreateRecipeResponse{
		Recipe: protoRecipe(created),
	}, nil
}

func (h *recipesConnectHandler) UpdateRecipe(
	ctx context.Context,
	req *recipesv1.UpdateRecipeRequest,
) (*recipesv1.UpdateRecipeResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid recipe ID",
		)
	}

	recipe, ingredients := dtoToRecipe(
		req.Name,
		req.Steps,
		req.BaseServings,
		req.IngredientNames,
		req.IngredientAmounts,
		req.IngredientUnits,
		req.IngredientGroupNames,
	)
	recipe.ID = id
	recipe.Ingredients = ingredients
	recipe.IsDraft = req.IsDraft
	if req.BatchServings != nil {
		v := int(*req.BatchServings)
		recipe.BatchServings = &v
	}

	err = h.app.services.Recipes.Update(ctx, user.ID, recipe)
	if err != nil {
		return nil, mapError(err)
	}

	return &recipesv1.UpdateRecipeResponse{}, nil
}

func (h *recipesConnectHandler) DeleteRecipe(
	ctx context.Context,
	req *recipesv1.DeleteRecipeRequest,
) (*recipesv1.DeleteRecipeResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid recipe ID",
		)
	}

	err = h.app.services.Recipes.Delete(ctx, id, user.ID)
	if err != nil {
		return nil, mapError(err)
	}

	return &recipesv1.DeleteRecipeResponse{}, nil
}
