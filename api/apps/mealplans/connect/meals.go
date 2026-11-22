package connect

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	"tools.xdoubleu.com/apps/mealplans/internal/protomap"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *Handler) CreateMeal(
	ctx context.Context,
	req *mealplansv1.CreateMealRequest,
) (*mealplansv1.CreateMealResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	mealDate, err := time.Parse(time.DateOnly, req.MealDate)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid meal date",
		)
	}

	var recipeID *uuid.UUID
	if req.RecipeId != "" {
		id, parseErr := uuid.Parse(req.RecipeId)
		if parseErr != nil {
			return nil, connect.Errorf(
				connect.CodeInvalidArgument,
				"invalid recipe ID",
			)
		}
		recipeID = &id
	}

	clientID, err := connecttools.ParseClientID(req.Id)
	if err != nil {
		return nil, err
	}

	meal := models.PlanMeal{ //nolint:exhaustruct // other fields optional
		ID:                      clientID.UUID,
		PlanID:                  planID,
		MealDate:                mealDate,
		MealSlot:                req.MealSlot,
		RecipeID:                recipeID,
		CustomName:              req.CustomName,
		Servings:                int(req.Servings),
		ExcludeFromShoppingList: req.ExcludeFromShoppingList,
	}

	if err = h.plans.CreateMeal(ctx, planID, uid, meal); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.CreateMealResponse{}, nil
}

func (h *Handler) UpdateMeal(
	ctx context.Context,
	req *mealplansv1.UpdateMealRequest,
) (*mealplansv1.UpdateMealResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	mealID, err := uuid.Parse(req.MealId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid meal ID",
		)
	}

	var recipeID *uuid.UUID
	if req.RecipeId != "" {
		id, parseErr := uuid.Parse(req.RecipeId)
		if parseErr != nil {
			return nil, connect.Errorf(
				connect.CodeInvalidArgument,
				"invalid recipe ID",
			)
		}
		recipeID = &id
	}

	meal := models.PlanMeal{ //nolint:exhaustruct // other fields untouched by update
		ID:                      mealID,
		PlanID:                  planID,
		RecipeID:                recipeID,
		CustomName:              req.CustomName,
		Servings:                int(req.Servings),
		ExcludeFromShoppingList: req.ExcludeFromShoppingList,
	}

	if err = h.plans.UpdateMeal(ctx, planID, uid, meal); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.UpdateMealResponse{}, nil
}

func (h *Handler) DeleteMeal(
	ctx context.Context,
	req *mealplansv1.DeleteMealRequest,
) (*mealplansv1.DeleteMealResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	mealID, err := uuid.Parse(req.MealId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid meal ID",
		)
	}

	if err = h.plans.DeleteMeal(ctx, mealID, planID, uid); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.DeleteMealResponse{}, nil
}

func (h *Handler) MoveMeal(
	ctx context.Context,
	req *mealplansv1.MoveMealRequest,
) (*mealplansv1.MoveMealResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	mealID, err := uuid.Parse(req.MealId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid meal ID",
		)
	}

	newDate, err := time.Parse(time.DateOnly, req.NewDate)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid meal date",
		)
	}

	if err = h.plans.MoveMeal(
		ctx, mealID, planID, uid, newDate, req.NewSlot,
	); err != nil {
		return nil, connecttools.MapError(err)
	}

	return &mealplansv1.MoveMealResponse{}, nil
}

func (h *Handler) SuggestRecipes(
	ctx context.Context,
	req *mealplansv1.SuggestRecipesRequest,
) (*mealplansv1.SuggestRecipesResponse, error) {
	uid, err := userID(ctx)
	if err != nil {
		return nil, err
	}

	planID, err := uuid.Parse(req.PlanId)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid plan ID",
		)
	}

	mealDate, err := time.Parse(time.DateOnly, req.MealDate)
	if err != nil {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid meal date",
		)
	}

	suggestions, err := h.plans.SuggestRecipes(
		ctx, planID, uid, mealDate, req.MealSlot,
	)
	if err != nil {
		return nil, connecttools.MapError(err)
	}

	return protomap.SuggestRecipes(suggestions), nil
}
