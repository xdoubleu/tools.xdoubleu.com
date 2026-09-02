package services_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/shoppinglist/internal/mocks"
	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
	"tools.xdoubleu.com/apps/shoppinglist/internal/services"
	iapp "tools.xdoubleu.com/internal/app"
)

var errNotFound = &iapp.HTTPError{
	Status:  http.StatusNotFound,
	Message: "Plan not found",
}

// fakeFamilyStore maps "user1" to a fixed family ID for tests.
type fakeFamilyStore struct {
	familyID uuid.UUID
	err      error
}

func (f *fakeFamilyStore) EnsureFamily(
	_ context.Context, _ string,
) (uuid.UUID, error) {
	if f.err != nil {
		return uuid.Nil, f.err
	}
	return f.familyID, nil
}

func newFamilyStore() *fakeFamilyStore {
	return &fakeFamilyStore{familyID: uuid.New(), err: nil}
}

func baseMock() *mocks.ShoppingRepoMock {
	return &mocks.ShoppingRepoMock{
		CheckPlanAccessFn:         nil,
		GetCustomItemsFn:          nil,
		AddCustomItemFn:           nil,
		UpdateCustomItemFn:        nil,
		DeleteCustomItemFn:        nil,
		GetMealPlanExportItemsFn:  nil,
		GetPlanIngredientGroupsFn: nil,
		ListCategoriesFn:          nil,
		CreateCategoryFn:          nil,
		RenameCategoryFn:          nil,
		DeleteCategoryFn:          nil,
		ListStoresFn:              nil,
		CreateStoreFn:             nil,
		RenameStoreFn:             nil,
		DeleteStoreFn:             nil,
		GetStoreCategoriesFn:      nil,
		SetStoreCategoriesFn:      nil,
		ListItemNamesFn:           nil,
		ListItemCategoriesFn:      nil,
		SetItemCategoryFn:         nil,
		SetItemExcludedFn:         nil,
	}
}

func accessGrantedMock() *mocks.ShoppingRepoMock {
	m := baseMock()
	m.CheckPlanAccessFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
		return nil
	}
	return m
}

func accessDeniedMock() *mocks.ShoppingRepoMock {
	m := baseMock()
	m.CheckPlanAccessFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
		return errNotFound
	}
	return m
}

func TestGetCustomList_ReturnsItems(t *testing.T) {
	want := []repositories.ShoppingItem{
		{ID: "id-1", Name: "milk", Unit: "L", Amount: 1, RecipeName: "", GroupName: ""},
		{ID: "id-2", Name: "eggs", Unit: "", Amount: 6, RecipeName: "", GroupName: ""},
	}
	family := newFamilyStore()
	m := baseMock()
	m.GetCustomItemsFn = func(
		_ context.Context, familyID uuid.UUID,
	) ([]repositories.ShoppingItem, error) {
		assert.Equal(t, family.familyID, familyID)
		return want, nil
	}

	svc := services.NewShoppingService(m, family)
	got, err := svc.GetCustomList(context.Background(), "user1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetCustomList_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.GetCustomItemsFn = func(
		_ context.Context, _ uuid.UUID,
	) ([]repositories.ShoppingItem, error) {
		return nil, repoErr
	}

	svc := services.NewShoppingService(m, newFamilyStore())
	_, err := svc.GetCustomList(context.Background(), "user1")
	assert.ErrorIs(t, err, repoErr)
}

func TestAddItem_Success(t *testing.T) {
	want := repositories.ShoppingItem{
		ID:         uuid.NewString(),
		Name:       "milk",
		Unit:       "L",
		Amount:     1,
		RecipeName: "",
		GroupName:  "",
	}
	family := newFamilyStore()
	m := baseMock()
	m.AddCustomItemFn = func(
		_ context.Context,
		familyID uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error) {
		assert.Equal(t, family.familyID, familyID)
		assert.Equal(t, "milk", name)
		assert.Equal(t, "L", unit)
		assert.InDelta(t, 1.0, amount, 1e-9)
		return want, nil
	}

	svc := services.NewShoppingService(m, family)
	got, err := svc.AddItem(context.Background(), "user1", "milk", "L", 1)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAddItem_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.AddCustomItemFn = func(
		_ context.Context, _ uuid.UUID, _, _ string, _ float64,
	) (repositories.ShoppingItem, error) {
		return repositories.ShoppingItem{}, repoErr
	}

	svc := services.NewShoppingService(m, newFamilyStore())
	_, err := svc.AddItem(context.Background(), "user1", "milk", "L", 1)
	assert.ErrorIs(t, err, repoErr)
}

func TestUpdateItem_Success(t *testing.T) {
	itemID := uuid.New()
	want := repositories.ShoppingItem{
		ID:         itemID.String(),
		Name:       "oat milk",
		Unit:       "L",
		Amount:     2,
		RecipeName: "",
		GroupName:  "",
	}
	family := newFamilyStore()
	m := baseMock()
	m.UpdateCustomItemFn = func(
		_ context.Context,
		familyID uuid.UUID,
		iID uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error) {
		assert.Equal(t, family.familyID, familyID)
		assert.Equal(t, itemID, iID)
		assert.Equal(t, "oat milk", name)
		assert.Equal(t, "L", unit)
		assert.InDelta(t, 2.0, amount, 1e-9)
		return want, nil
	}

	svc := services.NewShoppingService(m, family)
	got, err := svc.UpdateItem(
		context.Background(),
		"user1",
		itemID,
		"oat milk",
		"L",
		2,
	)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestUpdateItem_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.UpdateCustomItemFn = func(
		_ context.Context, _ uuid.UUID, _ uuid.UUID, _, _ string, _ float64,
	) (repositories.ShoppingItem, error) {
		return repositories.ShoppingItem{}, repoErr
	}

	svc := services.NewShoppingService(m, newFamilyStore())
	_, err := svc.UpdateItem(context.Background(), "user1", uuid.New(), "milk", "L", 1)
	assert.ErrorIs(t, err, repoErr)
}

func TestDeleteItem_Success(t *testing.T) {
	itemID := uuid.New()
	family := newFamilyStore()
	m := baseMock()
	m.DeleteCustomItemFn = func(
		_ context.Context, familyID uuid.UUID, iID uuid.UUID,
	) error {
		assert.Equal(t, family.familyID, familyID)
		assert.Equal(t, itemID, iID)
		return nil
	}

	svc := services.NewShoppingService(m, family)
	err := svc.DeleteItem(context.Background(), "user1", itemID)
	assert.NoError(t, err)
}

func TestDeleteItem_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.DeleteCustomItemFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
		return repoErr
	}

	svc := services.NewShoppingService(m, newFamilyStore())
	err := svc.DeleteItem(context.Background(), "user1", uuid.New())
	assert.ErrorIs(t, err, repoErr)
}

func TestSetItemExcluded_PassesThrough(t *testing.T) {
	family := newFamilyStore()
	m := baseMock()
	m.SetItemExcludedFn = func(
		_ context.Context, familyID uuid.UUID, name string, excluded bool,
	) error {
		assert.Equal(t, family.familyID, familyID)
		assert.Equal(t, "olive oil", name)
		assert.True(t, excluded)
		return nil
	}

	svc := services.NewShoppingService(m, family)
	err := svc.SetItemExcluded(context.Background(), "user1", "olive oil", true)
	assert.NoError(t, err)
}

func TestSetItemExcluded_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.SetItemExcludedFn = func(_ context.Context, _ uuid.UUID, _ string, _ bool) error {
		return repoErr
	}

	svc := services.NewShoppingService(m, newFamilyStore())
	err := svc.SetItemExcluded(context.Background(), "user1", "olive oil", false)
	assert.ErrorIs(t, err, repoErr)
}

func TestGetMealPlanExportItems_AccessDenied(t *testing.T) {
	svc := services.NewShoppingService(accessDeniedMock(), newFamilyStore())
	start := time.Now().UTC()
	_, err := svc.GetMealPlanExportItems(
		context.Background(),
		uuid.New(),
		"user1",
		start,
		start.AddDate(0, 0, 6),
		[]string{},
		[]string{},
	)
	assert.ErrorIs(t, err, errNotFound)
}

func TestGetMealPlanExportItems_Success(t *testing.T) {
	planID := uuid.New()
	start := time.Now().UTC()
	end := start.AddDate(0, 0, 6)
	pastSlots := []string{"breakfast"}
	planItems := []repositories.ShoppingItem{
		{
			ID:         "",
			Name:       "flour",
			Unit:       "g",
			Amount:     200,
			RecipeName: "Bread",
			GroupName:  "dry",
		},
	}
	m := accessGrantedMock()
	m.GetMealPlanExportItemsFn = func(
		_ context.Context, pID uuid.UUID, s, e time.Time, ps, _ []string,
	) ([]repositories.ShoppingItem, error) {
		assert.Equal(t, planID, pID)
		assert.Equal(t, start, s)
		assert.Equal(t, end, e)
		assert.Equal(t, pastSlots, ps)
		return planItems, nil
	}
	// Custom items must NOT be fetched here; the frontend merges them once on
	// its own. Fetching and appending would duplicate them per meal plan.
	m.GetCustomItemsFn = func(
		_ context.Context, _ uuid.UUID,
	) ([]repositories.ShoppingItem, error) {
		t.Fatal("GetCustomItems must not be called from GetMealPlanExportItems")
		return nil, nil
	}

	svc := services.NewShoppingService(m, newFamilyStore())
	got, err := svc.GetMealPlanExportItems(
		context.Background(),
		planID,
		"user1",
		start,
		end,
		pastSlots,
		[]string{},
	)
	require.NoError(t, err)
	assert.Equal(t, planItems, got)
}

func TestGetMealPlanExportItems_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := accessGrantedMock()
	m.GetMealPlanExportItemsFn = func(
		_ context.Context, _ uuid.UUID, _, _ time.Time, _, _ []string,
	) ([]repositories.ShoppingItem, error) {
		return nil, repoErr
	}

	svc := services.NewShoppingService(m, newFamilyStore())
	start := time.Now().UTC()
	_, err := svc.GetMealPlanExportItems(
		context.Background(),
		uuid.New(),
		"user1",
		start,
		start.AddDate(0, 0, 6),
		[]string{},
		[]string{},
	)
	assert.ErrorIs(t, err, repoErr)
}

// Every ShoppingService method resolves the caller's family before touching
// the repo; a family-resolution failure must propagate without calling the
// repo at all.
func TestFamilyResolutionErrors_PropagateWithoutTouchingRepo(t *testing.T) {
	familyErr := errors.New("family error")
	family := &fakeFamilyStore{familyID: uuid.Nil, err: familyErr}
	m := baseMock()
	svc := services.NewShoppingService(m, family)
	ctx := context.Background()

	_, err := svc.GetCustomList(ctx, "user1")
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.AddItem(ctx, "user1", "milk", "L", 1)
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.UpdateItem(ctx, "user1", uuid.New(), "milk", "L", 1)
	assert.ErrorIs(t, err, familyErr)

	err = svc.DeleteItem(ctx, "user1", uuid.New())
	assert.ErrorIs(t, err, familyErr)

	start := time.Now().UTC()
	_, err = svc.GetMealPlanExportItems(
		ctx, uuid.New(), "user1", start, start.AddDate(0, 0, 6), []string{}, []string{},
	)
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.GetPlanIngredientGroups(
		ctx, uuid.New(), "user1", start, start.AddDate(0, 0, 6), []string{},
	)
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.ListCategories(ctx, "user1")
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.CreateCategory(ctx, "user1", "Produce")
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.RenameCategory(ctx, "user1", uuid.New(), "Produce")
	assert.ErrorIs(t, err, familyErr)

	err = svc.DeleteCategory(ctx, "user1", uuid.New())
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.ListItemNames(ctx, "user1")
	assert.ErrorIs(t, err, familyErr)

	_, err = svc.ListItemCategories(ctx, "user1")
	assert.ErrorIs(t, err, familyErr)

	err = svc.SetItemCategory(ctx, "user1", "milk", uuid.New())
	assert.ErrorIs(t, err, familyErr)

	err = svc.SetItemExcluded(ctx, "user1", "milk", true)
	assert.ErrorIs(t, err, familyErr)

	err = svc.SetStoreCategories(ctx, "user1", uuid.New(), []uuid.UUID{})
	assert.ErrorIs(t, err, familyErr)
}
