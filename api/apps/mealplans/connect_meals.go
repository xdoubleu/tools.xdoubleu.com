package mealplans

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
)

func (h *mealplansConnectHandler) CreateMeal(
	ctx context.Context,
	req *connect.Request[mealplansv1.CreateMealRequest],
) (*connect.Response[mealplansv1.CreateMealResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	mealDate, err := time.Parse(time.DateOnly, req.Msg.MealDate)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal date"),
		)
	}

	var recipeID *uuid.UUID
	if req.Msg.RecipeId != "" {
		id, parseErr := uuid.Parse(req.Msg.RecipeId)
		if parseErr != nil {
			return nil, connect.NewError(
				connect.CodeInvalidArgument,
				fmt.Errorf("invalid recipe ID"),
			)
		}
		recipeID = &id
	}

	if recipeID == nil && req.Msg.CustomName == "" {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("recipe ID or custom name is required"),
		)
	}

	servings := 2
	if req.Msg.Servings > 0 {
		servings = int(req.Msg.Servings)
	}

	meal := models.PlanMeal{ //nolint:exhaustruct // other fields optional
		PlanID:                  planID,
		MealDate:                mealDate,
		MealSlot:                req.Msg.MealSlot,
		RecipeID:                recipeID,
		CustomName:              req.Msg.CustomName,
		Servings:                servings,
		ExcludeFromShoppingList: req.Msg.ExcludeFromShoppingList,
	}

	if err = h.app.services.Plans.CreateMeal(ctx, planID, user.ID, meal); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.CreateMealResponse{}), nil
}

func (h *mealplansConnectHandler) DeleteMeal(
	ctx context.Context,
	req *connect.Request[mealplansv1.DeleteMealRequest],
) (*connect.Response[mealplansv1.DeleteMealResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	mealID, err := uuid.Parse(req.Msg.MealId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal ID"),
		)
	}

	if err = h.app.services.Plans.DeleteMeal(ctx, mealID, planID, user.ID); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.DeleteMealResponse{}), nil
}

func (h *mealplansConnectHandler) MoveMeal(
	ctx context.Context,
	req *connect.Request[mealplansv1.MoveMealRequest],
) (*connect.Response[mealplansv1.MoveMealResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	mealID, err := uuid.Parse(req.Msg.MealId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal ID"),
		)
	}

	newDate, err := time.Parse(time.DateOnly, req.Msg.NewDate)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal date"),
		)
	}

	validSlots := map[string]bool{
		models.SlotBreakfast: true,
		models.SlotNoon:      true,
		models.SlotEvening:   true,
	}
	if !validSlots[req.Msg.NewSlot] {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal slot"),
		)
	}

	if err = h.app.services.Plans.MoveMeal(
		ctx, mealID, planID, user.ID, newDate, req.Msg.NewSlot,
	); err != nil {
		return nil, mapError(err)
	}

	return connect.NewResponse(&mealplansv1.MoveMealResponse{}), nil
}

func (h *mealplansConnectHandler) SuggestRecipes(
	ctx context.Context,
	req *connect.Request[mealplansv1.SuggestRecipesRequest],
) (*connect.Response[mealplansv1.SuggestRecipesResponse], error) {
	user := getUser(ctx)
	if user == nil {
		return nil, connect.NewError(
			connect.CodeUnauthenticated,
			fmt.Errorf("user not authenticated"),
		)
	}

	planID, err := uuid.Parse(req.Msg.PlanId)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid plan ID"),
		)
	}

	mealDate, err := time.Parse(time.DateOnly, req.Msg.MealDate)
	if err != nil {
		return nil, connect.NewError(
			connect.CodeInvalidArgument,
			fmt.Errorf("invalid meal date"),
		)
	}

	ids, err := h.app.services.Plans.SuggestRecipes(
		ctx, planID, user.ID, mealDate, req.Msg.MealSlot,
	)
	if err != nil {
		return nil, mapError(err)
	}

	recipeIDs := make([]string, len(ids))
	for i, id := range ids {
		recipeIDs[i] = id.String()
	}

	return connect.NewResponse(&mealplansv1.SuggestRecipesResponse{
		RecipeIds: recipeIDs,
	}), nil
}
