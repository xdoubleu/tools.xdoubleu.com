package shoppinglist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
)

// createTestPlan inserts a minimal meal plan owned by the test user and returns its ID.
func createTestPlan(t *testing.T, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO mealplans.plans (owner_user_id, name)
		VALUES ($1, $2) RETURNING id`,
		userID, name,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// deletePlan removes the plan after the test.
func deletePlan(t *testing.T, planID uuid.UUID) {
	t.Helper()
	_, err := testDB.Exec(
		context.Background(),
		"DELETE FROM mealplans.plans WHERE id = $1",
		planID,
	)
	require.NoError(t, err)
}

func newShoppingClient(t *testing.T) shoppinglistv1connect.ShoppingListServiceClient {
	t.Helper()
	ts := httptest.NewServer(getRoutes())
	t.Cleanup(ts.Close)
	return shoppinglistv1connect.NewShoppingListServiceClient(
		http.DefaultClient,
		ts.URL,
	)
}

// ── GetCustomList ─────────────────────────────────────────────────────────────

func TestGetCustomList_Empty(t *testing.T) {
	client := newShoppingClient(t)
	resp, err := client.GetCustomList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetCustomListRequest{}),
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Items)
}

// ── AddShoppingItem ───────────────────────────────────────────────────────────

func TestAddShoppingItem_Success(t *testing.T) {
	client := newShoppingClient(t)
	resp, err := client.AddShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.AddShoppingItemRequest{
			Name:   "Milk",
			Amount: "2",
			Unit:   "L",
		}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Item.Id)
	assert.Equal(t, "Milk", resp.Msg.Item.Name)
	assert.Equal(t, "L", resp.Msg.Item.Unit)
}

func TestAddShoppingItem_EmptyName(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.AddShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.AddShoppingItemRequest{
			Name:   "",
			Amount: "1",
			Unit:   "kg",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestAddShoppingItem_InvalidAmount(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.AddShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.AddShoppingItemRequest{
			Name:   "Eggs",
			Amount: "not-a-number",
			Unit:   "pcs",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// ── DeleteShoppingItem ────────────────────────────────────────────────────────

func TestDeleteShoppingItem_Success(t *testing.T) {
	client := newShoppingClient(t)

	addResp, err := client.AddShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.AddShoppingItemRequest{
			Name:   "Butter",
			Amount: "1",
			Unit:   "block",
		}),
	)
	require.NoError(t, err)

	_, err = client.DeleteShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.DeleteShoppingItemRequest{
			ItemId: addResp.Msg.Item.Id,
		}),
	)
	require.NoError(t, err)
}

func TestDeleteShoppingItem_NotFound(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.DeleteShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.DeleteShoppingItemRequest{
			ItemId: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestDeleteShoppingItem_InvalidID(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.DeleteShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.DeleteShoppingItemRequest{
			ItemId: "not-a-uuid",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// ── GetCustomList — non-empty ─────────────────────────────────────────────────

func TestGetCustomList_WithItems(t *testing.T) {
	client := newShoppingClient(t)

	_, err := client.AddShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.AddShoppingItemRequest{
			Name:   "Cheese",
			Amount: "200",
			Unit:   "g",
		}),
	)
	require.NoError(t, err)

	resp, err := client.GetCustomList(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetCustomListRequest{}),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Msg.Items)
}

// ── GetMealPlanExportItems ────────────────────────────────────────────────────

func TestGetMealPlanExportItems_InvalidPlanID(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.GetMealPlanExportItems(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetMealPlanExportItemsRequest{
			PlanId: "not-a-uuid",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestGetMealPlanExportItems_PlanNotFound(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.GetMealPlanExportItems(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetMealPlanExportItemsRequest{
			PlanId: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestGetMealPlanExportItems_Success(t *testing.T) {
	planID := createTestPlan(t, "Test Plan")
	t.Cleanup(func() { deletePlan(t, planID) })

	client := newShoppingClient(t)
	resp, err := client.GetMealPlanExportItems(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetMealPlanExportItemsRequest{
			PlanId: planID.String(),
		}),
	)
	require.NoError(t, err)
	// No meals added, so day items should be empty.
	assert.NotNil(t, resp.Msg)
}

func TestGetMealPlanExportItems_IncludesCustomItems(t *testing.T) {
	planID := createTestPlan(t, "Plan With Custom")
	t.Cleanup(func() { deletePlan(t, planID) })

	client := newShoppingClient(t)

	// Add a custom shopping list item for the user.
	addResp, err := client.AddShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.AddShoppingItemRequest{
			Name:   "Olive Oil",
			Amount: "1",
			Unit:   "bottle",
		}),
	)
	require.NoError(t, err)
	customItemID := addResp.Msg.Item.Id
	t.Cleanup(func() {
		_, _ = client.DeleteShoppingItem(
			t.Context(),
			connect.NewRequest(&shoppinglistv1.DeleteShoppingItemRequest{
				ItemId: customItemID,
			}),
		)
	})

	resp, err := client.GetMealPlanExportItems(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetMealPlanExportItemsRequest{
			PlanId: planID.String(),
		}),
	)
	require.NoError(t, err)

	names := make([]string, 0, len(resp.Msg.Items))
	for _, item := range resp.Msg.Items {
		names = append(names, item.Name)
	}
	assert.Contains(t, names, "Olive Oil")
}

// createTestRecipeWithGroups inserts a recipe with two grouped ingredients and
// returns the recipe ID.
func createTestRecipeWithGroups(t *testing.T) uuid.UUID {
	t.Helper()
	var recipeID uuid.UUID
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, name, instructions, base_servings)
		VALUES ($1, 'Spaghetti', '', 2) RETURNING id`,
		userID,
	).Scan(&recipeID)
	require.NoError(t, err)

	_, err = testDB.Exec(context.Background(), `
		INSERT INTO recipes.ingredients
		(recipe_id, name, amount, unit, sort_order, group_name)
		VALUES
		($1, 'tomatoes',   200, 'g', 0, 'sauce'),
		($1, 'garlic',     2,   '',  1, 'sauce'),
		($1, 'spaghetti',  100, 'g', 2, 'pasta')`,
		recipeID,
	)
	require.NoError(t, err)
	return recipeID
}

// addPlanMeal links a recipe to a plan for the given date and slot.
func addPlanMeal(
	t *testing.T,
	planID, recipeID uuid.UUID,
	mealDate time.Time,
	slot string,
) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		INSERT INTO mealplans.plan_meals (plan_id, meal_date, meal_slot, recipe_id, servings)
		VALUES ($1, $2, $3, $4, 2)`,
		planID, mealDate.Format("2006-01-02"), slot, recipeID,
	)
	require.NoError(t, err)
}

// ── GetPlanIngredientGroups ───────────────────────────────────────────────────

func TestGetPlanIngredientGroups_InvalidPlanID(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.GetPlanIngredientGroups(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetPlanIngredientGroupsRequest{
			PlanId: "not-a-uuid",
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestGetPlanIngredientGroups_PlanNotFound(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.GetPlanIngredientGroups(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetPlanIngredientGroupsRequest{
			PlanId: uuid.New().String(),
		}),
	)
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestGetPlanIngredientGroups_ReturnsGroups(t *testing.T) {
	planID := createTestPlan(t, "Groups Plan")
	t.Cleanup(func() { deletePlan(t, planID) })

	recipeID := createTestRecipeWithGroups(t)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			"DELETE FROM recipes.recipes WHERE id = $1",
			recipeID,
		)
	})

	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	addPlanMeal(t, planID, recipeID, tomorrow, "noon")

	client := newShoppingClient(t)
	resp, err := client.GetPlanIngredientGroups(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetPlanIngredientGroupsRequest{
			PlanId: planID.String(),
		}),
	)
	require.NoError(t, err)

	groupNames := make([]string, 0, len(resp.Msg.Groups))
	for _, g := range resp.Msg.Groups {
		groupNames = append(groupNames, g.GroupName)
	}
	assert.Contains(t, groupNames, "sauce")
	assert.Contains(t, groupNames, "pasta")
}

// ── GetMealPlanExportItems with recipe and group attribution ──────────────────

func TestGetMealPlanExportItems_RecipeAndGroupAttribution(t *testing.T) {
	planID := createTestPlan(t, "Attribution Plan")
	t.Cleanup(func() { deletePlan(t, planID) })

	recipeID := createTestRecipeWithGroups(t)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			"DELETE FROM recipes.recipes WHERE id = $1",
			recipeID,
		)
	})

	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	addPlanMeal(t, planID, recipeID, tomorrow, "noon")

	client := newShoppingClient(t)
	resp, err := client.GetMealPlanExportItems(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetMealPlanExportItemsRequest{
			PlanId: planID.String(),
		}),
	)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Items, "expected meal-plan items in response")

	// Index items by name for targeted assertions.
	byName := make(map[string]*shoppinglistv1.ShoppingItem, len(resp.Msg.Items))
	for _, item := range resp.Msg.Items {
		byName[item.Name] = item
	}

	// Each ingredient from the Spaghetti recipe must carry the recipe name and
	// its ingredient group.
	for _, tc := range []struct {
		name  string
		group string
	}{
		{"tomatoes", "sauce"},
		{"garlic", "sauce"},
		{"spaghetti", "pasta"},
	} {
		item, ok := byName[tc.name]
		if !assert.True(t, ok, "item %q not found in response", tc.name) {
			continue
		}
		assert.Equal(t, "Spaghetti", item.RecipeName, "item %q RecipeName", tc.name)
		assert.Equal(t, tc.group, item.GroupName, "item %q GroupName", tc.name)
	}
}

// ── GetMealPlanExportItems with group exclusion ───────────────────────────────

func TestGetMealPlanExportItems_ExcludesGroup(t *testing.T) {
	planID := createTestPlan(t, "Exclude Group Plan")
	t.Cleanup(func() { deletePlan(t, planID) })

	recipeID := createTestRecipeWithGroups(t)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			"DELETE FROM recipes.recipes WHERE id = $1",
			recipeID,
		)
	})

	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	addPlanMeal(t, planID, recipeID, tomorrow, "noon")

	client := newShoppingClient(t)

	// Exclude the sauce group — only pasta ingredients should appear.
	resp, err := client.GetMealPlanExportItems(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetMealPlanExportItemsRequest{
			PlanId:         planID.String(),
			ExcludedGroups: []string{"sauce"},
		}),
	)
	require.NoError(t, err)

	names := make([]string, 0, len(resp.Msg.Items))
	for _, item := range resp.Msg.Items {
		names = append(names, item.Name)
	}
	assert.Contains(t, names, "spaghetti")
	assert.NotContains(t, names, "tomatoes")
	assert.NotContains(t, names, "garlic")
}
