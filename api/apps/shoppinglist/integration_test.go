package shoppinglist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
