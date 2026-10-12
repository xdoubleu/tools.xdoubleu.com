package mealplans

import (
	"context"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/internal/connecttools"
)

func (h *mealplansConnectHandler) CreateMeal(
	ctx context.Context,
	req *mealplansv1.CreateMealRequest,
) (*mealplansv1.CreateMealResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
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

	if recipeID == nil && req.CustomName == "" {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"recipe ID or custom name is required",
		)
	}

	servings := 2
	if req.Servings > 0 {
		servings = int(req.Servings)
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
		Servings:                servings,
		ExcludeFromShoppingList: req.ExcludeFromShoppingList,
	}

	if err = h.app.services.Plans.CreateMeal(ctx, planID, user.ID, meal); err != nil {
		return nil, mapError(err)
	}

	return &mealplansv1.CreateMealResponse{}, nil
}

func (h *mealplansConnectHandler) UpdateMeal(
	ctx context.Context,
	req *mealplansv1.UpdateMealRequest,
) (*mealplansv1.UpdateMealResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
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

	if recipeID == nil && req.CustomName == "" {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"recipe ID or custom name is required",
		)
	}

	servings := 2
	if req.Servings > 0 {
		servings = int(req.Servings)
	}

	meal := models.PlanMeal{ //nolint:exhaustruct // other fields untouched by update
		ID:                      mealID,
		PlanID:                  planID,
		RecipeID:                recipeID,
		CustomName:              req.CustomName,
		Servings:                servings,
		ExcludeFromShoppingList: req.ExcludeFromShoppingList,
	}

	if err = h.app.services.Plans.UpdateMeal(ctx, planID, user.ID, meal); err != nil {
		return nil, mapError(err)
	}

	return &mealplansv1.UpdateMealResponse{}, nil
}

func (h *mealplansConnectHandler) DeleteMeal(
	ctx context.Context,
	req *mealplansv1.DeleteMealRequest,
) (*mealplansv1.DeleteMealResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
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

	if err = h.app.services.Plans.DeleteMeal(ctx, mealID, planID, user.ID); err != nil {
		return nil, mapError(err)
	}

	return &mealplansv1.DeleteMealResponse{}, nil
}

func (h *mealplansConnectHandler) MoveMeal(
	ctx context.Context,
	req *mealplansv1.MoveMealRequest,
) (*mealplansv1.MoveMealResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
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

	validSlots := map[string]bool{
		models.SlotBreakfast: true,
		models.SlotNoon:      true,
		models.SlotEvening:   true,
	}
	if !validSlots[req.NewSlot] {
		return nil, connect.Errorf(
			connect.CodeInvalidArgument,
			"invalid meal slot",
		)
	}

	if err = h.app.services.Plans.MoveMeal(
		ctx, mealID, planID, user.ID, newDate, req.NewSlot,
	); err != nil {
		return nil, mapError(err)
	}

	return &mealplansv1.MoveMealResponse{}, nil
}

func (h *mealplansConnectHandler) SuggestRecipes(
	ctx context.Context,
	req *mealplansv1.SuggestRecipesRequest,
) (*mealplansv1.SuggestRecipesResponse, error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.Errorf(
			connect.CodeUnauthenticated,
			"user not authenticated",
		)
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

	suggestions, err := h.app.services.Plans.SuggestRecipes(
		ctx, planID, user.ID, mealDate, req.MealSlot,
	)
	if err != nil {
		return nil, mapError(err)
	}

	pbSuggestions := make([]*mealplansv1.RecipeSuggestion, len(suggestions))
	for i, s := range suggestions {
		pbSuggestions[i] = &mealplansv1.RecipeSuggestion{
			RecipeId: s.RecipeID.String(),
			Servings: int32(s.Servings), //nolint:gosec // int32 safe for domain values
		}
	}

	return &mealplansv1.SuggestRecipesResponse{
		Suggestions: pbSuggestions,
	}, nil
}
