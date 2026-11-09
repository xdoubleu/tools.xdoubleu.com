package mealplans_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func TestGetPlan_KeepsSlotOrderAfterUpdate(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	planID := createPlanInDB(t, "Stable Order Plan")
	date := time.Now().UTC().Format(time.DateOnly)

	// Descending ids, so ordering by id alone would reverse the creation order.
	ids := []string{
		"ffffffff-0000-4000-8000-000000000003",
		"88888888-0000-4000-8000-000000000002",
		"00000000-0000-4000-8000-000000000001",
	}
	for _, id := range ids {
		_, err := client.CreateMeal(ctx, &mealplansv1.CreateMealRequest{
			PlanId: planID, MealDate: date, MealSlot: "noon",
			CustomName: "item " + id, Servings: 1, Id: id,
		})
		require.NoError(t, err)
	}

	mealIDs := func() []string {
		resp, err := client.GetPlan(ctx, &mealplansv1.GetPlanRequest{Id: planID})
		require.NoError(t, err)
		var got []string
		for _, m := range resp.Plan.Meals {
			got = append(got, m.Id)
		}
		return got
	}
	assert.Equal(t, ids, mealIDs())

	// Each update rewrites the row, which moves it in heap order.
	for i := len(ids) - 1; i >= 0; i-- {
		_, err := client.UpdateMeal(ctx, &mealplansv1.UpdateMealRequest{
			PlanId: planID, MealId: ids[i], CustomName: "changed", Servings: 1,
		})
		require.NoError(t, err)
	}
	assert.Equal(t, ids, mealIDs())
}
