package shoppinglist_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
)

func createCategory(
	t *testing.T,
	client shoppinglistv1connect.ShoppingListServiceClient,
	name string,
) *shoppinglistv1.Category {
	t.Helper()
	resp, err := client.CreateCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.CreateCategoryRequest{Name: name}),
	)
	require.NoError(t, err)
	return resp.Msg.Category
}

func createStore(
	t *testing.T,
	client shoppinglistv1connect.ShoppingListServiceClient,
	name string,
) *shoppinglistv1.Store {
	t.Helper()
	resp, err := client.CreateStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.CreateStoreRequest{Name: name}),
	)
	require.NoError(t, err)
	return resp.Msg.Store
}

func assertCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, code, connectErr.Code())
}

// ── Categories ────────────────────────────────────────────────────────────────

func TestCreateCategory_Success(t *testing.T) {
	client := newShoppingClient(t)
	c := createCategory(t, client, "Produce")
	assert.NotEmpty(t, c.Id)
	assert.Equal(t, "Produce", c.Name)
}

func TestCreateCategory_EmptyName(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.CreateCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.CreateCategoryRequest{Name: ""}),
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestCreateCategory_DuplicateNameConflict(t *testing.T) {
	client := newShoppingClient(t)
	createCategory(t, client, "Dairy")
	_, err := client.CreateCategory(
		t.Context(),
		// Case-insensitive unique index: "dairy" collides with "Dairy".
		connect.NewRequest(&shoppinglistv1.CreateCategoryRequest{Name: "dairy"}),
	)
	assertCode(t, err, connect.CodeAlreadyExists)
}

func TestListCategories_ReturnsCreated(t *testing.T) {
	client := newShoppingClient(t)
	created := createCategory(t, client, "Bakery-"+uuid.NewString())

	resp, err := client.ListCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListCategoriesRequest{}),
	)
	require.NoError(t, err)
	var found bool
	for _, c := range resp.Msg.Categories {
		if c.Id == created.Id {
			found = true
		}
	}
	assert.True(t, found)
}

func TestRenameCategory_Success(t *testing.T) {
	client := newShoppingClient(t)
	c := createCategory(t, client, "Froozen-"+uuid.NewString())

	resp, err := client.RenameCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.RenameCategoryRequest{
			Id:   c.Id,
			Name: "Frozen-" + uuid.NewString(),
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, c.Id, resp.Msg.Category.Id)
}

func TestRenameCategory_NotFound(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.RenameCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.RenameCategoryRequest{
			Id:   uuid.New().String(),
			Name: "Whatever",
		}),
	)
	assertCode(t, err, connect.CodeNotFound)
}

func TestRenameCategory_InvalidID(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.RenameCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.RenameCategoryRequest{
			Id:   "not-a-uuid",
			Name: "Whatever",
		}),
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestDeleteCategory_Success(t *testing.T) {
	client := newShoppingClient(t)
	c := createCategory(t, client, "Temp-"+uuid.NewString())

	_, err := client.DeleteCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.DeleteCategoryRequest{Id: c.Id}),
	)
	require.NoError(t, err)
}

func TestDeleteCategory_NotFound(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.DeleteCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.DeleteCategoryRequest{
			Id: uuid.New().String(),
		}),
	)
	assertCode(t, err, connect.CodeNotFound)
}

// ── Stores & ordering ─────────────────────────────────────────────────────────

func TestCreateStore_Success(t *testing.T) {
	client := newShoppingClient(t)
	s := createStore(t, client, "Colruyt-"+uuid.NewString())
	assert.NotEmpty(t, s.Id)
}

func TestCreateStore_EmptyName(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.CreateStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.CreateStoreRequest{Name: ""}),
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestListStores_ReturnsCreated(t *testing.T) {
	client := newShoppingClient(t)
	created := createStore(t, client, "Jumbo-"+uuid.NewString())

	resp, err := client.ListStores(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListStoresRequest{}),
	)
	require.NoError(t, err)
	var found bool
	for _, s := range resp.Msg.Stores {
		if s.Id == created.Id {
			found = true
		}
	}
	assert.True(t, found)
}

func TestRenameStore_InvalidID(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.RenameStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.RenameStoreRequest{
			Id:   "not-a-uuid",
			Name: "Whatever",
		}),
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestRenameStore_NotFound(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.RenameStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.RenameStoreRequest{
			Id:   uuid.New().String(),
			Name: "Whatever",
		}),
	)
	assertCode(t, err, connect.CodeNotFound)
}

func TestSetStoreCategories_SkipsUnknownCategory(t *testing.T) {
	client := newShoppingClient(t)
	store := createStore(t, client, "Okay-"+uuid.NewString())

	// A syntactically valid but non-existent category id is silently skipped.
	_, err := client.SetStoreCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetStoreCategoriesRequest{
			StoreId:     store.Id,
			CategoryIds: []string{uuid.New().String()},
		}),
	)
	require.NoError(t, err)

	resp, err := client.GetStoreCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetStoreCategoriesRequest{
			StoreId: store.Id,
		}),
	)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Categories)
}

func TestRenameAndDeleteStore(t *testing.T) {
	client := newShoppingClient(t)
	s := createStore(t, client, "Aldi-"+uuid.NewString())

	_, err := client.RenameStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.RenameStoreRequest{
			Id:   s.Id,
			Name: "Lidl-" + uuid.NewString(),
		}),
	)
	require.NoError(t, err)

	_, err = client.DeleteStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.DeleteStoreRequest{Id: s.Id}),
	)
	require.NoError(t, err)

	_, err = client.DeleteStore(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.DeleteStoreRequest{Id: s.Id}),
	)
	assertCode(t, err, connect.CodeNotFound)
}

func TestSetAndGetStoreCategories_OrderPreserved(t *testing.T) {
	client := newShoppingClient(t)
	store := createStore(t, client, "Delhaize-"+uuid.NewString())
	c1 := createCategory(t, client, "Veg-"+uuid.NewString())
	c2 := createCategory(t, client, "Meat-"+uuid.NewString())
	c3 := createCategory(t, client, "Drinks-"+uuid.NewString())

	// Set order c2, c3, c1.
	_, err := client.SetStoreCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetStoreCategoriesRequest{
			StoreId:     store.Id,
			CategoryIds: []string{c2.Id, c3.Id, c1.Id},
		}),
	)
	require.NoError(t, err)

	resp, err := client.GetStoreCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetStoreCategoriesRequest{
			StoreId: store.Id,
		}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Categories, 3)
	assert.Equal(t, c2.Id, resp.Msg.Categories[0].Id)
	assert.Equal(t, c3.Id, resp.Msg.Categories[1].Id)
	assert.Equal(t, c1.Id, resp.Msg.Categories[2].Id)
}

func TestSetStoreCategories_ReplacesPrevious(t *testing.T) {
	client := newShoppingClient(t)
	store := createStore(t, client, "Carrefour-"+uuid.NewString())
	c1 := createCategory(t, client, "A-"+uuid.NewString())
	c2 := createCategory(t, client, "B-"+uuid.NewString())

	set := func(ids ...string) {
		_, err := client.SetStoreCategories(
			t.Context(),
			connect.NewRequest(&shoppinglistv1.SetStoreCategoriesRequest{
				StoreId:     store.Id,
				CategoryIds: ids,
			}),
		)
		require.NoError(t, err)
	}

	set(c1.Id, c2.Id)
	set(c2.Id) // replace: only c2 should remain

	resp, err := client.GetStoreCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetStoreCategoriesRequest{
			StoreId: store.Id,
		}),
	)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Categories, 1)
	assert.Equal(t, c2.Id, resp.Msg.Categories[0].Id)
}

func TestGetStoreCategories_StoreNotFound(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.GetStoreCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.GetStoreCategoriesRequest{
			StoreId: uuid.New().String(),
		}),
	)
	assertCode(t, err, connect.CodeNotFound)
}

func TestSetStoreCategories_InvalidCategoryID(t *testing.T) {
	client := newShoppingClient(t)
	store := createStore(t, client, "Spar-"+uuid.NewString())
	_, err := client.SetStoreCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetStoreCategoriesRequest{
			StoreId:     store.Id,
			CategoryIds: []string{"not-a-uuid"},
		}),
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

// ── Item catalog ──────────────────────────────────────────────────────────────

func TestSetItemCategory_AssignAndList(t *testing.T) {
	client := newShoppingClient(t)
	cat := createCategory(t, client, "Spices-"+uuid.NewString())

	// Mixed-case name is normalized to lower(trim) on write.
	_, err := client.SetItemCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetItemCategoryRequest{
			Name:       "  Pepper  ",
			CategoryId: cat.Id,
		}),
	)
	require.NoError(t, err)

	resp, err := client.ListItemCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListItemCategoriesRequest{}),
	)
	require.NoError(t, err)
	var got string
	for _, ic := range resp.Msg.Items {
		if ic.Name == "pepper" {
			got = ic.CategoryId
		}
	}
	assert.Equal(t, cat.Id, got)
}

func TestSetItemCategory_Unassign(t *testing.T) {
	client := newShoppingClient(t)
	cat := createCategory(t, client, "Cans-"+uuid.NewString())

	assign := func(categoryID string) {
		_, err := client.SetItemCategory(
			t.Context(),
			connect.NewRequest(&shoppinglistv1.SetItemCategoryRequest{
				Name:       "beans",
				CategoryId: categoryID,
			}),
		)
		require.NoError(t, err)
	}
	assign(cat.Id)
	assign("") // empty clears the mapping

	resp, err := client.ListItemCategories(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListItemCategoriesRequest{}),
	)
	require.NoError(t, err)
	for _, ic := range resp.Msg.Items {
		assert.NotEqual(t, "beans", ic.Name)
	}
}

func TestSetItemCategory_UnknownCategory(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.SetItemCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetItemCategoryRequest{
			Name:       "salt",
			CategoryId: uuid.New().String(),
		}),
	)
	assertCode(t, err, connect.CodeNotFound)
}

func TestSetItemCategory_EmptyName(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.SetItemCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetItemCategoryRequest{
			Name:       "",
			CategoryId: uuid.New().String(),
		}),
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestListItemNames_IncludesCustomItemsWithAssignment(t *testing.T) {
	client := newShoppingClient(t)
	cat := createCategory(t, client, "Frozen-"+uuid.NewString())

	itemName := "icecream-" + uuid.NewString()
	addResp, err := client.AddShoppingItem(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.AddShoppingItemRequest{
			Name:   itemName,
			Amount: "1",
			Unit:   "tub",
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			"DELETE FROM shoppinglist.custom_items WHERE id::text = $1",
			addResp.Msg.Item.Id,
		)
	})

	_, err = client.SetItemCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetItemCategoryRequest{
			Name:       itemName,
			CategoryId: cat.Id,
		}),
	)
	require.NoError(t, err)

	resp, err := client.ListItemNames(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListItemNamesRequest{}),
	)
	require.NoError(t, err)
	var got string
	var found bool
	for _, n := range resp.Msg.Names {
		if n.Name == itemName {
			found = true
			got = n.CategoryId
		}
	}
	require.True(t, found)
	assert.Equal(t, cat.Id, got)
}

// Custom (recipe-less) meal-plan entries store hand-typed item names in
// custom_name. Those names must surface in the item catalog so they can be
// categorized; otherwise they always export as uncategorized.
func TestListItemNames_IncludesMealPlanCustomItems(t *testing.T) {
	planID := createTestPlan(t, "Catalog Plan "+uuid.NewString())
	t.Cleanup(func() { deletePlan(t, planID) })

	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	// Two newline-separated items; the second carries a tab amount that must be
	// stripped so only the bare name reaches the catalog.
	plain := "tortillas-" + uuid.NewString()
	withAmount := "salsa-" + uuid.NewString()
	addCustomPlanMeal(t, planID, tomorrow, "noon", plain+"\n"+withAmount+"\t2")

	client := newShoppingClient(t)
	resp, err := client.ListItemNames(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListItemNamesRequest{}),
	)
	require.NoError(t, err)

	byName := make(map[string]string, len(resp.Msg.Names))
	for _, n := range resp.Msg.Names {
		byName[n.Name] = n.CategoryId
	}
	catID, plainFound := byName[plain]
	require.True(t, plainFound, "plain meal-plan custom name missing from catalog")
	assert.Empty(t, catID, "unassigned name should have empty category")
	_, amountFound := byName[withAmount]
	require.True(
		t,
		amountFound,
		"tab-amount meal-plan custom name missing from catalog",
	)

	// The surfaced name is categorizable, and the assignment is reflected back.
	cat := createCategory(t, client, "Mexican-"+uuid.NewString())
	_, err = client.SetItemCategory(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.SetItemCategoryRequest{
			Name:       plain,
			CategoryId: cat.Id,
		}),
	)
	require.NoError(t, err)

	resp, err = client.ListItemNames(
		t.Context(),
		connect.NewRequest(&shoppinglistv1.ListItemNamesRequest{}),
	)
	require.NoError(t, err)
	var got string
	for _, n := range resp.Msg.Names {
		if n.Name == plain {
			got = n.CategoryId
		}
	}
	assert.Equal(t, cat.Id, got)
}
