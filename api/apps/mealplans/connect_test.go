package mealplans_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mealplansv1 "tools.xdoubleu.com/gen/mealplans/v1"
	"tools.xdoubleu.com/gen/mealplans/v1/mealplansv1connect"
	"tools.xdoubleu.com/internal/constants"
	sharedmodels "tools.xdoubleu.com/internal/models"
)

func setupMealPlansClient(
	handler http.Handler,
) mealplansv1connect.MealPlansServiceClient {
	ts := httptest.NewServer(handler)
	return mealplansv1connect.NewMealPlansServiceClient(http.DefaultClient, ts.URL)
}

func contextWithUser(ctx context.Context, user *sharedmodels.User) context.Context {
	return context.WithValue(ctx, constants.UserContextKey, user)
}

// createRecipeInDB inserts a minimal recipe directly so mealplans tests can
// reference a real recipe_id without spinning up the recipes app.
func createRecipeInDB(t *testing.T, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, name, base_servings)
		VALUES ($1, $2, 2)
		RETURNING id`,
		userID, name,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func connectErr(err error) *connect.Error {
	target := &connect.Error{}
	_ = errors.As(err, &target)
	return target
}

func TestListPlans_Empty(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	resp, err := client.ListPlans(
		ctx,
		connect.NewRequest(&mealplansv1.ListPlansRequest{}),
	)
	require.NoError(t, err)
	assert.Equal(t, 0, len(resp.Msg.Plans))
}

func TestCreatePlan_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	resp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Weekly Plan",
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "Weekly Plan", resp.Msg.Plan.Name)
	assert.Equal(t, userID, resp.Msg.Plan.OwnerUserId)
}

func TestGetPlan_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Test Plan",
		}),
	)
	require.NoError(t, err)
	planID := createResp.Msg.Plan.Id

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	assert.Equal(t, "Test Plan", getResp.Msg.Plan.Name)
	assert.True(t, getResp.Msg.IsOwner)
	assert.NotEmpty(t, getResp.Msg.IcalUrl)
	assert.Equal(t, int32(0), getResp.Msg.Offset)
}

func TestUpdatePlan_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Original Plan",
		}),
	)
	require.NoError(t, err)
	planID := createResp.Msg.Plan.Id

	_, err = client.UpdatePlan(ctx, connect.NewRequest(&mealplansv1.UpdatePlanRequest{
		Id:            planID,
		Name:          "Updated Plan",
		IcalHideSlots: []string{"breakfast"},
		IcalHidePast:  true,
	}))
	require.NoError(t, err)

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	assert.Equal(t, "Updated Plan", getResp.Msg.Plan.Name)
}

func TestDeletePlan_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Plan to Delete",
		}),
	)
	require.NoError(t, err)
	planID := createResp.Msg.Plan.Id

	_, err = client.DeletePlan(
		ctx,
		connect.NewRequest(&mealplansv1.DeletePlanRequest{Id: planID}),
	)
	require.NoError(t, err)

	_, err = client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestAddMeal_WithRecipe(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	recipeID := createRecipeInDB(t, "Chicken")

	planResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Week Plan",
		}),
	)
	require.NoError(t, err)
	planID := planResp.Msg.Plan.Id

	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId:   planID,
		MealDate: time.Now().Format("2006-01-02"),
		MealSlot: "noon",
		RecipeId: recipeID.String(),
		Servings: 4,
	}))
	require.NoError(t, err)

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	assert.Equal(t, 1, len(getResp.Msg.Plan.Meals))
	assert.Equal(t, "noon", getResp.Msg.Plan.Meals[0].MealSlot)
	assert.NotEmpty(t, getResp.Msg.Plan.Meals[0].RecipeId)
}

func TestAddMeal_WithCustomName(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	planResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Custom Meals Plan",
		}),
	)
	require.NoError(t, err)
	planID := planResp.Msg.Plan.Id

	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId:     planID,
		MealDate:   time.Now().Format("2006-01-02"),
		MealSlot:   "breakfast",
		CustomName: "Scrambled eggs",
		Servings:   1,
	}))
	require.NoError(t, err)

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	assert.Equal(t, 1, len(getResp.Msg.Plan.Meals))
	assert.Equal(t, "breakfast", getResp.Msg.Plan.Meals[0].MealSlot)
	assert.Equal(t, "Scrambled eggs", getResp.Msg.Plan.Meals[0].CustomName)
	assert.Equal(t, "", getResp.Msg.Plan.Meals[0].RecipeId)
}

func TestAddMeal_RequiresRecipeOrName(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	planResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Test Plan",
		}),
	)
	require.NoError(t, err)
	planID := planResp.Msg.Plan.Id

	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId: planID, MealDate: time.Now().Format("2006-01-02"),
		MealSlot: "noon", Servings: 2,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestDeleteMeal_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	planResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Delete Meal Plan",
		}),
	)
	require.NoError(t, err)
	planID := planResp.Msg.Plan.Id

	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId:     planID,
		MealDate:   time.Now().Format("2006-01-02"),
		MealSlot:   "noon",
		CustomName: "Test Meal",
		Servings:   2,
	}))
	require.NoError(t, err)

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(getResp.Msg.Plan.Meals))
	mealID := getResp.Msg.Plan.Meals[0].Id

	_, err = client.DeleteMeal(ctx, connect.NewRequest(&mealplansv1.DeleteMealRequest{
		PlanId: planID, MealId: mealID,
	}))
	require.NoError(t, err)

	getResp2, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	assert.Equal(t, 0, len(getResp2.Msg.Plan.Meals))
}

func TestSharePlan_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Share Plan",
		}),
	)
	require.NoError(t, err)
	planID := createResp.Msg.Plan.Id

	shareResp, err := client.SharePlan(
		ctx,
		connect.NewRequest(&mealplansv1.SharePlanRequest{
			PlanId:        planID,
			ContactUserId: "other-user",
			CanEdit:       true,
		}),
	)
	require.NoError(t, err)
	_ = shareResp
}

func TestUnsharePlan_RequiresTargetUserID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Unshare Plan",
		}),
	)
	require.NoError(t, err)
	planID := createResp.Msg.Plan.Id

	_, err = client.UnsharePlan(ctx, connect.NewRequest(&mealplansv1.UnsharePlanRequest{
		PlanId: planID, TargetUserId: "",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestMoveMeal_ToEmptySlot(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	planResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Move Test Plan",
		}),
	)
	require.NoError(t, err)
	planID := planResp.Msg.Plan.Id

	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId: planID, MealDate: today, MealSlot: "noon",
		CustomName: "Pasta", Servings: 2,
	}))
	require.NoError(t, err)

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	mealID := getResp.Msg.Plan.Meals[0].Id

	_, err = client.MoveMeal(ctx, connect.NewRequest(&mealplansv1.MoveMealRequest{
		PlanId: planID, MealId: mealID, NewDate: tomorrow, NewSlot: "noon",
	}))
	require.NoError(t, err)

	getResp, err = client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(getResp.Msg.Plan.Meals))
	assert.Equal(t, tomorrow, getResp.Msg.Plan.Meals[0].MealDate)
	assert.Equal(t, "Pasta", getResp.Msg.Plan.Meals[0].CustomName)
}

func TestMoveMeal_SwapTwoMeals(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	planResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "Swap Test Plan",
		}),
	)
	require.NoError(t, err)
	planID := planResp.Msg.Plan.Id

	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId: planID, MealDate: today, MealSlot: "noon",
		CustomName: "Pasta", Servings: 2,
	}))
	require.NoError(t, err)
	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId: planID, MealDate: tomorrow, MealSlot: "noon",
		CustomName: "Salad", Servings: 1,
	}))
	require.NoError(t, err)

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	require.Equal(t, 2, len(getResp.Msg.Plan.Meals))

	var pastaID string
	for _, m := range getResp.Msg.Plan.Meals {
		if m.CustomName == "Pasta" {
			pastaID = m.Id
		}
	}
	require.NotEmpty(t, pastaID)

	_, err = client.MoveMeal(ctx, connect.NewRequest(&mealplansv1.MoveMealRequest{
		PlanId: planID, MealId: pastaID, NewDate: tomorrow, NewSlot: "noon",
	}))
	require.NoError(t, err)

	getResp, err = client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	require.Equal(t, 2, len(getResp.Msg.Plan.Meals))

	byDate := map[string]string{}
	for _, m := range getResp.Msg.Plan.Meals {
		byDate[m.MealDate] = m.CustomName
	}
	assert.Equal(t, "Pasta", byDate[tomorrow])
	assert.Equal(t, "Salad", byDate[today])
}

func TestMoveMeal_NoOp(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	planResp, err := client.CreatePlan(
		ctx,
		connect.NewRequest(&mealplansv1.CreatePlanRequest{
			Name: "NoOp Test Plan",
		}),
	)
	require.NoError(t, err)
	planID := planResp.Msg.Plan.Id

	today := time.Now().Format("2006-01-02")

	_, err = client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId: planID, MealDate: today, MealSlot: "evening",
		CustomName: "Soup", Servings: 2,
	}))
	require.NoError(t, err)

	getResp, err := client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	mealID := getResp.Msg.Plan.Meals[0].Id

	_, err = client.MoveMeal(ctx, connect.NewRequest(&mealplansv1.MoveMealRequest{
		PlanId: planID, MealId: mealID, NewDate: today, NewSlot: "evening",
	}))
	require.NoError(t, err)

	getResp, err = client.GetPlan(ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{
		Id: planID, Offset: 0,
	}))
	require.NoError(t, err)
	require.Equal(t, 1, len(getResp.Msg.Plan.Meals))
	assert.Equal(t, today, getResp.Msg.Plan.Meals[0].MealDate)
	assert.Equal(t, "evening", getResp.Msg.Plan.Meals[0].MealSlot)
}

func TestUnsharePlan_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreatePlan(
		ctx, connect.NewRequest(&mealplansv1.CreatePlanRequest{Name: "Unshare Test"}),
	)
	require.NoError(t, err)
	planID := createResp.Msg.Plan.Id

	_, err = client.SharePlan(ctx, connect.NewRequest(&mealplansv1.SharePlanRequest{
		PlanId: planID, ContactUserId: "other-user", CanEdit: false,
	}))
	require.NoError(t, err)

	_, err = client.UnsharePlan(ctx, connect.NewRequest(&mealplansv1.UnsharePlanRequest{
		PlanId: planID, TargetUserId: "other-user",
	}))
	require.NoError(t, err)
}

func TestDeletePlan_NotFound(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.DeletePlan(ctx, connect.NewRequest(&mealplansv1.DeletePlanRequest{
		Id: uuid.New().String(),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestDeleteMeal_InvalidPlanID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.DeleteMeal(ctx, connect.NewRequest(&mealplansv1.DeleteMealRequest{
		PlanId: "not-a-uuid", MealId: uuid.New().String(),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestMoveMeal_InvalidPlanID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.MoveMeal(ctx, connect.NewRequest(&mealplansv1.MoveMealRequest{
		PlanId: "not-a-uuid", MealId: uuid.New().String(),
		NewDate: "2026-01-01", NewSlot: "noon",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestICalFeedHandler_InvalidToken(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/mealplans/ical/not-a-token.ics")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestICalFeedHandler_TokenNotFound(t *testing.T) {
	ts := httptest.NewServer(getRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/mealplans/ical/" + uuid.New().String() + ".ics")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestICalFeedHandler_Success(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)

	createResp, err := client.CreatePlan(
		ctx, connect.NewRequest(&mealplansv1.CreatePlanRequest{Name: "iCal Plan"}),
	)
	require.NoError(t, err)
	planID := createResp.Msg.Plan.Id

	getResp, err := client.GetPlan(
		ctx, connect.NewRequest(&mealplansv1.GetPlanRequest{Id: planID, Offset: 0}),
	)
	require.NoError(t, err)
	icalURL := getResp.Msg.IcalUrl

	ts := httptest.NewServer(getRoutes())
	defer ts.Close()

	resp, err := http.Get(ts.URL + icalURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/calendar")
}

func TestListPlans_WithItems(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.CreatePlan(
		ctx, connect.NewRequest(&mealplansv1.CreatePlanRequest{Name: "List Test Plan"}),
	)
	require.NoError(t, err)

	resp, err := client.ListPlans(
		ctx, connect.NewRequest(&mealplansv1.ListPlansRequest{}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Plans)
}

func TestUpdatePlan_InvalidPlanID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.UpdatePlan(ctx, connect.NewRequest(&mealplansv1.UpdatePlanRequest{
		Id: "not-a-uuid", Name: "x",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestUpdatePlan_NotFound(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.UpdatePlan(ctx, connect.NewRequest(&mealplansv1.UpdatePlanRequest{
		Id: uuid.New().String(), Name: "ghost",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connectErr(err).Code())
}

func TestAddMeal_InvalidPlanID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId: "not-a-uuid", MealDate: "2026-01-01", MealSlot: "noon", CustomName: "x",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestAddMeal_InvalidDate(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId: uuid.New().
			String(),
		MealDate: "not-a-date", MealSlot: "noon", CustomName: "x",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestAddMeal_InvalidRecipeID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.AddMeal(ctx, connect.NewRequest(&mealplansv1.AddMealRequest{
		PlanId:   uuid.New().String(),
		MealDate: "2026-01-01",
		MealSlot: "noon",
		RecipeId: "not-a-uuid",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestDeleteMeal_InvalidMealID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.DeleteMeal(ctx, connect.NewRequest(&mealplansv1.DeleteMealRequest{
		PlanId: uuid.New().String(), MealId: "not-a-uuid",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestMoveMeal_InvalidMealID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.MoveMeal(ctx, connect.NewRequest(&mealplansv1.MoveMealRequest{
		PlanId: uuid.New().String(), MealId: "not-a-uuid",
		NewDate: "2026-01-01", NewSlot: "noon",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestMoveMeal_InvalidDate(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.MoveMeal(ctx, connect.NewRequest(&mealplansv1.MoveMealRequest{
		PlanId: uuid.New().String(), MealId: uuid.New().String(),
		NewDate: "not-a-date", NewSlot: "noon",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestMoveMeal_InvalidSlot(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.MoveMeal(ctx, connect.NewRequest(&mealplansv1.MoveMealRequest{
		PlanId: uuid.New().String(), MealId: uuid.New().String(),
		NewDate: "2026-01-01", NewSlot: "invalid-slot",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestSharePlan_InvalidPlanID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.SharePlan(ctx, connect.NewRequest(&mealplansv1.SharePlanRequest{
		PlanId: "not-a-uuid", ContactUserId: "other-user",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}

func TestUnsharePlan_InvalidPlanID(t *testing.T) {
	client := setupMealPlansClient(getRoutes())
	ctx := contextWithUser(
		context.Background(),
		&sharedmodels.User{ //nolint:exhaustruct // only ID needed
			ID: userID,
		},
	)
	_, err := client.UnsharePlan(
		ctx,
		connect.NewRequest(&mealplansv1.UnsharePlanRequest{
			PlanId: "not-a-uuid", TargetUserId: "other-user",
		}),
	)
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr(err).Code())
}
