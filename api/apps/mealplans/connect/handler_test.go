package connect_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mealplansconnect "tools.xdoubleu.com/apps/mealplans/connect"
	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/internal/kit/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func TestRotateICalToken_Unauthenticated(t *testing.T) {
	h := mealplansconnect.NewHandler(nil)
	_, err := h.RotateICalToken(
		context.Background(),
		&mealplansv1.RotateICalTokenRequest{Id: "x"},
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func authedCtx() context.Context {
	return context.WithValue(
		context.Background(),
		constants.UserContextKey,
		sharedmodels.User{ID: "u"}, //nolint:exhaustruct // only ID needed
	)
}

func TestHandler_MealRPCsUnauthenticated(t *testing.T) {
	h := mealplansconnect.NewHandler(nil)
	ctx := context.Background()

	_, err := h.CreateMeal(ctx, &mealplansv1.CreateMealRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = h.UpdateMeal(ctx, &mealplansv1.UpdateMealRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = h.DeleteMeal(ctx, &mealplansv1.DeleteMealRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = h.MoveMeal(ctx, &mealplansv1.MoveMealRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = h.SuggestRecipes(ctx, &mealplansv1.SuggestRecipesRequest{})
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestHandler_MealRPCsInvalidIDs(t *testing.T) {
	h := mealplansconnect.NewHandler(nil)
	ctx := authedCtx()
	valid := uuid.NewString()

	_, err := h.CreateMeal(ctx, &mealplansv1.CreateMealRequest{PlanId: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.CreateMeal(ctx, &mealplansv1.CreateMealRequest{
		PlanId: valid, MealDate: "nope",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.CreateMeal(ctx, &mealplansv1.CreateMealRequest{
		PlanId: valid, MealDate: "2026-01-02", RecipeId: "x",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = h.UpdateMeal(ctx, &mealplansv1.UpdateMealRequest{PlanId: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.UpdateMeal(ctx, &mealplansv1.UpdateMealRequest{
		PlanId: valid, MealId: "x",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.UpdateMeal(ctx, &mealplansv1.UpdateMealRequest{
		PlanId: valid, MealId: valid, RecipeId: "x",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = h.DeleteMeal(ctx, &mealplansv1.DeleteMealRequest{PlanId: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.DeleteMeal(ctx, &mealplansv1.DeleteMealRequest{
		PlanId: valid, MealId: "x",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = h.MoveMeal(ctx, &mealplansv1.MoveMealRequest{PlanId: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.MoveMeal(ctx, &mealplansv1.MoveMealRequest{PlanId: valid, MealId: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.MoveMeal(ctx, &mealplansv1.MoveMealRequest{
		PlanId: valid, MealId: valid, NewDate: "nope",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = h.SuggestRecipes(ctx, &mealplansv1.SuggestRecipesRequest{PlanId: "x"})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.SuggestRecipes(ctx, &mealplansv1.SuggestRecipesRequest{
		PlanId: valid, MealDate: "nope",
	})
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
