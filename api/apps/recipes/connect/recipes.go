package connect

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/recipes/internal/protomap"
	"tools.xdoubleu.com/apps/recipes/internal/services"
	recipesv1 "tools.xdoubleu.com/gen/recipes/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *Handler) ListRecipes(
	ctx context.Context,
	req *recipesv1.ListRecipesRequest,
) (*recipesv1.ListRecipesResponse, error) {
	userID, err := CallerID(ctx)
	if err != nil {
		return nil, err
	}

	list, hasMore, err := h.recipes.List(
		ctx, userID, req.MealType, req.Effort, req.Limit, req.Offset,
	)
	if err != nil {
		return nil, protomap.MapError(err)
	}

	return &recipesv1.ListRecipesResponse{
		Recipes: protomap.Recipes(list),
		HasMore: hasMore,
	}, nil
}

func (h *Handler) GetRecipe(
	ctx context.Context,
	req *recipesv1.GetRecipeRequest,
) (*recipesv1.GetRecipeResponse, error) {
	userID, err := CallerID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid recipe ID",
		)
	}

	recipe, canEdit, servings, scaledIngredients, err := h.recipes.GetScaled(
		ctx,
		id,
		userID,
		int(req.Servings),
	)
	if err != nil {
		return nil, protomap.MapError(err)
	}

	return protomap.ScaledRecipeResponse(userID, &services.ScaledRecipe{
		Recipe: recipe, CanEdit: canEdit, Servings: servings, Ingredients: scaledIngredients,
	}), nil
}

func (h *Handler) CreateRecipe(
	ctx context.Context,
	req *recipesv1.CreateRecipeRequest,
) (*recipesv1.CreateRecipeResponse, error) {
	userID, err := CallerID(ctx)
	if err != nil {
		return nil, err
	}

	clientID, err := connecttools.ParseClientID(req.Id)
	if err != nil {
		return nil, err
	}
	in := recipeInput(
		clientID.UUID, req.Name, req.Steps, req.BaseServings, req.BatchServings,
		req.IsDraft, req.MealType, req.Effort, req.IngredientNames,
		req.IngredientAmounts, req.IngredientUnits, req.IngredientGroupNames,
		req.IngredientAlternativeGroups,
	)

	created, err := h.recipes.Create(ctx, userID, in)
	if err != nil {
		return nil, protomap.MapError(err)
	}

	return &recipesv1.CreateRecipeResponse{
		Recipe: protomap.Recipe(created),
	}, nil
}

func (h *Handler) UpdateRecipe(
	ctx context.Context,
	req *recipesv1.UpdateRecipeRequest,
) (*recipesv1.UpdateRecipeResponse, error) {
	userID, err := CallerID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid recipe ID",
		)
	}

	in := recipeInput(
		id, req.Name, req.Steps, req.BaseServings, req.BatchServings,
		req.IsDraft, req.MealType, req.Effort, req.IngredientNames,
		req.IngredientAmounts, req.IngredientUnits, req.IngredientGroupNames,
		req.IngredientAlternativeGroups,
	)

	err = h.recipes.Update(ctx, userID, in)
	if err != nil {
		return nil, protomap.MapError(err)
	}

	return &recipesv1.UpdateRecipeResponse{}, nil
}

func (h *Handler) DeleteRecipe(
	ctx context.Context,
	req *recipesv1.DeleteRecipeRequest,
) (*recipesv1.DeleteRecipeResponse, error) {
	userID, err := CallerID(ctx)
	if err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid recipe ID",
		)
	}

	err = h.recipes.Delete(ctx, id, userID)
	if err != nil {
		return nil, protomap.MapError(err)
	}

	return &recipesv1.DeleteRecipeResponse{}, nil
}
