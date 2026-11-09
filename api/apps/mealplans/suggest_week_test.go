package mealplans_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func TestSuggestWeek_FillsEmptySlotsWithUsualMeal(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	recipeA := createRecipeInDB(t, "Week A")
	recipeB := createRecipeInDB(t, "Week B")
	planID := createPlanInDB(t, "Suggest Week Plan")

	add := func(date, slot string, recipe uuid.UUID, servings int32) {
		_, err := client.CreateMeal(ctx, &mealplansv1.CreateMealRequest{
			PlanId: planID, MealDate: date, MealSlot: slot,
			RecipeId: recipe.String(), Servings: servings,
		})
		require.NoError(t, err)
	}

	// Mondays at noon: A twice (servings 4), B once -> A wins.
	add("2024-01-01", "noon", recipeA, 4)
	add("2024-01-08", "noon", recipeA, 4)
	add("2024-01-15", "noon", recipeB, 2)
	// Tuesday evening has history, but the target week already has a meal there.
	add("2024-01-02", "evening", recipeB, 2)
	add("2024-02-06", "evening", recipeA, 1)

	resp, err := client.SuggestWeek(ctx, &mealplansv1.SuggestWeekRequest{
		PlanId: planID, StartDate: "2024-02-05", // a Monday
	})
	require.NoError(t, err)
	require.Len(t, resp.Suggestions, 1)
	got := resp.Suggestions[0]
	assert.Equal(t, "2024-02-05", got.MealDate)
	assert.Equal(t, "noon", got.MealSlot)
	assert.Equal(t, recipeA.String(), got.RecipeId)
	assert.Equal(t, int32(4), got.Servings)
}

func TestSuggestWeek_InvalidStartDate(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	planID := createPlanInDB(t, "Suggest Week Invalid")

	_, err := client.SuggestWeek(ctx, &mealplansv1.SuggestWeekRequest{
		PlanId: planID, StartDate: "nope",
	})
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}
