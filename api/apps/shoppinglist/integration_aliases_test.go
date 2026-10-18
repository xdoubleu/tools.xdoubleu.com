package shoppinglist_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	shoppinglistv1 "tools.xdoubleu.com/gen/shoppinglist/v1"
	"tools.xdoubleu.com/gen/shoppinglist/v1/shoppinglistv1connect"
)

type testIngredient struct {
	name   string
	amount float64
	unit   string
}

// createRecipe inserts a recipe (base servings 2) removed after the test.
func createRecipe(t *testing.T, ingredients ...testIngredient) uuid.UUID {
	t.Helper()
	familyID, err := familyRepo.EnsureFamily(context.Background(), userID)
	require.NoError(t, err)

	var recipeID uuid.UUID
	err = testDB.QueryRow(context.Background(), `
		INSERT INTO recipes.recipes (user_id, family_id, name, instructions, base_servings)
		VALUES ($1, $2, $3, '', 2) RETURNING id`,
		userID, familyID, "recipe-"+uuid.NewString(),
	).Scan(&recipeID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			"DELETE FROM recipes.recipes WHERE id = $1",
			recipeID,
		)
	})

	for i, ing := range ingredients {
		_, err = testDB.Exec(context.Background(), `
			INSERT INTO recipes.ingredients (recipe_id, name, amount, unit, sort_order)
			VALUES ($1, $2, $3, $4, $5)`,
			recipeID, ing.name, ing.amount, ing.unit, i,
		)
		require.NoError(t, err)
	}
	return recipeID
}

// mergeItem merges name into canonical and unmerges it after the test.
func mergeItem(
	t *testing.T,
	client shoppinglistv1connect.ShoppingListServiceClient,
	name, canonical string,
) {
	t.Helper()
	_, err := client.SetItemAlias(
		t.Context(),
		&shoppinglistv1.SetItemAliasRequest{Name: name, Canonical: canonical},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			"DELETE FROM shoppinglist.item_aliases WHERE alias = $1",
			name,
		)
	})
}

func exportItems(
	t *testing.T,
	client shoppinglistv1connect.ShoppingListServiceClient,
	planID uuid.UUID,
) []*shoppinglistv1.ShoppingItem {
	t.Helper()
	resp, err := client.GetMealPlanExportItems(
		t.Context(),
		&shoppinglistv1.GetMealPlanExportItemsRequest{PlanId: planID.String()},
	)
	require.NoError(t, err)
	return resp.Items
}

func TestSetItemAlias_MergesExportNames(t *testing.T) {
	suffix := uuid.NewString()
	paprika := "paprika-" + suffix
	rode := "rode paprika-" + suffix

	planID := createTestPlan(t, "Alias Plan "+suffix)
	t.Cleanup(func() { deletePlan(t, planID) })
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	first := createRecipe(t, testIngredient{"Rode Paprika-" + suffix, 1, "stuk"})
	second := createRecipe(t, testIngredient{paprika, 200, "g"})
	addPlanMeal(t, planID, first, tomorrow, "noon")
	addPlanMeal(t, planID, second, tomorrow, "evening")
	addCustomPlanMeal(t, planID, tomorrow, "breakfast", rode+"\t2")

	client := newShoppingClient(t)
	mergeItem(t, client, rode, paprika)

	var names []string
	for _, item := range exportItems(t, client, planID) {
		names = append(names, item.Name)
	}
	assert.Equal(t, []string{paprika, paprika, paprika}, names)

	entry := catalogEntry(t, client, rode)
	require.NotNil(t, entry)
	assert.Equal(t, paprika, entry.MergedInto)
	canonical := catalogEntry(t, client, paprika)
	require.NotNil(t, canonical)
	assert.Empty(t, canonical.MergedInto)
}

// A canonical with no source of its own still surfaces while an alias exports
// as it.
func TestListItemNames_SurfacesCanonicalOfActiveAlias(t *testing.T) {
	suffix := uuid.NewString()
	alias := "ui-rood-" + suffix
	canonical := "ui-" + suffix
	createRecipe(t, testIngredient{alias, 1, ""})

	client := newShoppingClient(t)
	mergeItem(t, client, alias, canonical)

	entry := catalogEntry(t, client, canonical)
	require.NotNil(t, entry)
	assert.False(t, entry.Excluded)
}

func TestSetItemAlias_FlattensChains(t *testing.T) {
	suffix := uuid.NewString()
	a, b, c, d := "a-"+suffix, "b-"+suffix, "c-"+suffix, "d-"+suffix
	createRecipe(t,
		testIngredient{a, 1, ""}, testIngredient{b, 1, ""},
		testIngredient{c, 1, ""}, testIngredient{d, 1, ""},
	)

	client := newShoppingClient(t)
	mergeItem(t, client, a, b)
	// Merging the canonical moves its aliases along.
	mergeItem(t, client, b, c)
	// Merging into an alias targets its canonical.
	mergeItem(t, client, d, a)

	for _, name := range []string{a, b, d} {
		entry := catalogEntry(t, client, name)
		require.NotNil(t, entry, name)
		assert.Equal(t, c, entry.MergedInto, name)
	}
}

func TestSetItemAlias_RejectsSelfMerge(t *testing.T) {
	suffix := uuid.NewString()
	a, b := "a-"+suffix, "b-"+suffix
	client := newShoppingClient(t)

	_, err := client.SetItemAlias(
		t.Context(),
		&shoppinglistv1.SetItemAliasRequest{Name: a, Canonical: " " + a + " "},
	)
	assertCode(t, err, connect.CodeInvalidArgument)

	// b's canonical resolves to a's alias target, which is b itself.
	mergeItem(t, client, a, b)
	_, err = client.SetItemAlias(
		t.Context(),
		&shoppinglistv1.SetItemAliasRequest{Name: b, Canonical: a},
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestSetItemAlias_EmptyName(t *testing.T) {
	client := newShoppingClient(t)
	_, err := client.SetItemAlias(
		t.Context(),
		&shoppinglistv1.SetItemAliasRequest{Name: " ", Canonical: "x"},
	)
	assertCode(t, err, connect.CodeInvalidArgument)
}

func TestSetItemAlias_EmptyCanonicalUnmerges(t *testing.T) {
	suffix := uuid.NewString()
	a, b := "a-"+suffix, "b-"+suffix
	createRecipe(t, testIngredient{a, 1, ""})

	client := newShoppingClient(t)
	mergeItem(t, client, a, b)
	_, err := client.SetItemAlias(
		t.Context(),
		&shoppinglistv1.SetItemAliasRequest{Name: a},
	)
	require.NoError(t, err)

	entry := catalogEntry(t, client, a)
	require.NotNil(t, entry)
	assert.Empty(t, entry.MergedInto)
	assert.Nil(t, catalogEntry(t, client, b), "canonical without sources drops out")
}

func TestSetItemAlias_CanonicalInheritsCategoryOnlyWhenUnassigned(t *testing.T) {
	suffix := uuid.NewString()
	alias, bare, assigned := "alias-"+suffix, "bare-"+suffix, "assigned-"+suffix
	createRecipe(t,
		testIngredient{alias, 1, ""}, testIngredient{bare, 1, ""},
		testIngredient{assigned, 1, ""},
	)

	client := newShoppingClient(t)
	groenten := createCategory(t, client, "Groenten-"+suffix)
	other := createCategory(t, client, "Other-"+suffix)
	assignments := map[string]string{alias: groenten.Id, assigned: other.Id}
	for name, categoryID := range assignments {
		_, err := client.SetItemCategory(
			t.Context(),
			&shoppinglistv1.SetItemCategoryRequest{Name: name, CategoryId: categoryID},
		)
		require.NoError(t, err)
	}

	mergeItem(t, client, alias, bare)
	assert.Equal(t, groenten.Id, catalogEntry(t, client, bare).CategoryId)

	mergeItem(t, client, bare, assigned)
	assert.Equal(t, other.Id, catalogEntry(t, client, assigned).CategoryId)
}

func TestSetItemExcluded_AppliesToAliases(t *testing.T) {
	suffix := uuid.NewString()
	alias, canonical := "alias-"+suffix, "canonical-"+suffix
	planID := createTestPlan(t, "Alias Exclude Plan "+suffix)
	t.Cleanup(func() { deletePlan(t, planID) })
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	addCustomPlanMeal(t, planID, tomorrow, "noon", alias)
	addCustomPlanMeal(t, planID, tomorrow, "evening", canonical)

	client := newShoppingClient(t)
	added, err := client.CreateShoppingItem(
		t.Context(),
		&shoppinglistv1.CreateShoppingItemRequest{Name: alias, Amount: "1"},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = testDB.Exec(
			context.Background(),
			"DELETE FROM shoppinglist.custom_items WHERE id::text = $1",
			added.Item.Id,
		)
	})
	mergeItem(t, client, alias, canonical)

	setExcluded := func(excluded bool) {
		_, err = client.SetItemExcluded(
			t.Context(),
			&shoppinglistv1.SetItemExcludedRequest{Name: canonical, Excluded: excluded},
		)
		require.NoError(t, err)
	}

	setExcluded(true)
	assert.Empty(t, exportItems(t, client, planID))
	assert.True(t, catalogEntry(t, client, alias).Excluded)
	custom, err := client.GetCustomList(
		t.Context(),
		&shoppinglistv1.GetCustomListRequest{},
	)
	require.NoError(t, err)
	for _, item := range custom.Items {
		assert.NotEqual(t, alias, item.Name, "aliased custom item should be deleted")
	}

	setExcluded(false)
	assert.Len(t, exportItems(t, client, planID), 1)
	assert.False(t, catalogEntry(t, client, alias).Excluded)
}

func TestListIngredientUnits_GroupsByMergedName(t *testing.T) {
	suffix := uuid.NewString()
	paprika, rode := "paprika-"+suffix, "rode paprika-"+suffix
	first := createRecipe(t, testIngredient{paprika, 1, "stuk"})
	second := createRecipe(t, testIngredient{"Rode Paprika-" + suffix, 2, " stuk "})
	third := createRecipe(t, testIngredient{paprika, 200, "g"})

	client := newShoppingClient(t)
	mergeItem(t, client, rode, paprika)

	resp, err := client.ListIngredientUnits(
		t.Context(),
		&shoppinglistv1.ListIngredientUnitsRequest{},
	)
	require.NoError(t, err)

	byUnit := map[string][]string{}
	for _, u := range resp.Units {
		if u.Name == paprika {
			byUnit[u.Unit] = u.RecipeIds
		}
		assert.NotEqual(t, rode, u.Name, "aliases resolve to their canonical")
	}
	assert.Equal(t, []string{third.String()}, byUnit["g"])
	assert.ElementsMatch(t, []string{first.String(), second.String()}, byUnit["stuk"])
}
