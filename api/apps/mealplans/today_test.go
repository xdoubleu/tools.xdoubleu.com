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

func TestListTodayMeals_TodayAndTomorrowEvening(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	// The page lists a user's newest plans only; start from a clean slate.
	_, err := testDB.Exec(context.Background(),
		"DELETE FROM mealplans.plans WHERE owner_user_id = $1", userID)
	require.NoError(t, err)
	planID := createPlanInDB(t, "Today Plan")

	now := time.Now()
	for _, m := range []struct {
		date       time.Time
		slot, name string
	}{
		{now, "noon", "today-lunch"},
		{now.AddDate(0, 0, 1), "evening", "tomorrow-dinner"},
		{now.AddDate(0, 0, 1), "breakfast", "tomorrow-breakfast"},
	} {
		_, err = client.CreateMeal(ctx, &mealplansv1.CreateMealRequest{
			PlanId:     planID,
			MealDate:   m.date.Format("2006-01-02"),
			MealSlot:   m.slot,
			CustomName: m.name,
			Servings:   1,
		})
		require.NoError(t, err)
	}

	meals, err := testApp.ListTodayMeals(context.Background(), userID, now)
	require.NoError(t, err)

	names := make([]string, 0, len(meals))
	for _, m := range meals {
		names = append(names, m.Name)
	}
	assert.Contains(t, names, "today-lunch")
	assert.Contains(t, names, "tomorrow-dinner")
	assert.NotContains(t, names, "tomorrow-breakfast")
}
