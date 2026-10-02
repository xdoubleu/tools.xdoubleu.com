package mealplans_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func userCtx() context.Context {
	return contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
}

func customMeal(planID, id string) *mealplansv1.CreateMealRequest {
	return &mealplansv1.CreateMealRequest{
		Id:         id,
		PlanId:     planID,
		MealDate:   time.Now().Format("2006-01-02"),
		MealSlot:   "evening",
		CustomName: "Offline pasta",
		Servings:   2,
	}
}

func TestCreateMeal_ClientIDRepeatIsIdempotent(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	planID := createPlanInDB(t, "Offline Plan")
	id := uuid.NewString()

	for range 2 {
		_, err := client.CreateMeal(userCtx(), connect.NewRequest(customMeal(planID, id)))
		require.NoError(t, err)
	}

	got, err := client.GetPlan(userCtx(), connect.NewRequest(
		&mealplansv1.GetPlanRequest{Id: planID},
	))
	require.NoError(t, err)
	require.Len(t, got.Msg.Plan.Meals, 1)
	assert.Equal(t, id, got.Msg.Plan.Meals[0].Id)
}

func TestCreateMeal_ClientIDReplayKeepsLaterEdits(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	planID := createPlanInDB(t, "Offline Edit Plan")
	id := uuid.NewString()

	_, err := client.CreateMeal(userCtx(), connect.NewRequest(customMeal(planID, id)))
	require.NoError(t, err)
	update := &mealplansv1.UpdateMealRequest{
		PlanId: planID, MealId: id, CustomName: "Renamed", Servings: 4,
	}
	_, err = client.UpdateMeal(userCtx(), connect.NewRequest(update))
	require.NoError(t, err)
	_, err = client.CreateMeal(userCtx(), connect.NewRequest(customMeal(planID, id)))
	require.NoError(t, err)

	got, err := client.GetPlan(userCtx(), connect.NewRequest(
		&mealplansv1.GetPlanRequest{Id: planID},
	))
	require.NoError(t, err)
	require.Len(t, got.Msg.Plan.Meals, 1)
	assert.Equal(t, "Renamed", got.Msg.Plan.Meals[0].CustomName)
	assert.Equal(t, int32(4), got.Msg.Plan.Meals[0].Servings)
}

func TestCreateMeal_ClientIDOnAnotherPlan(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	first := createPlanInDB(t, "First Plan")
	second := createPlanInDB(t, "Second Plan")
	id := uuid.NewString()

	_, err := client.CreateMeal(userCtx(), connect.NewRequest(customMeal(first, id)))
	require.NoError(t, err)
	_, err = client.CreateMeal(userCtx(), connect.NewRequest(customMeal(second, id)))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestCreateMeal_InvalidClientID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	planID := createPlanInDB(t, "Invalid ID Plan")

	_, err := client.CreateMeal(userCtx(), connect.NewRequest(customMeal(planID, "nope")))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}
