package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
)

type ShoppingRepoMock struct {
	CheckPlanAccessFn func(
		ctx context.Context,
		planID uuid.UUID,
		userID string,
	) error
	GetCustomItemsFn func(
		ctx context.Context,
		userID string,
	) ([]repositories.ShoppingItem, error)
	AddCustomItemFn func(
		ctx context.Context,
		userID, name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error)
	DeleteCustomItemFn func(
		ctx context.Context, userID string, itemID uuid.UUID,
	) error
	GetMealPlanExportItemsFn func(
		ctx context.Context,
		planID uuid.UUID,
		start, end time.Time,
		pastSlots []string,
	) ([]repositories.ShoppingItem, error)
	ListCategoriesFn func(
		ctx context.Context, userID string,
	) ([]repositories.Category, error)
	CreateCategoryFn func(
		ctx context.Context, userID, name string,
	) (repositories.Category, error)
	RenameCategoryFn func(
		ctx context.Context, userID string, id uuid.UUID, name string,
	) (repositories.Category, error)
	DeleteCategoryFn func(ctx context.Context, userID string, id uuid.UUID) error
	ListStoresFn     func(
		ctx context.Context, userID string,
	) ([]repositories.Store, error)
	CreateStoreFn func(
		ctx context.Context, userID, name string,
	) (repositories.Store, error)
	RenameStoreFn func(
		ctx context.Context, userID string, id uuid.UUID, name string,
	) (repositories.Store, error)
	DeleteStoreFn        func(ctx context.Context, userID string, id uuid.UUID) error
	GetStoreCategoriesFn func(
		ctx context.Context, userID string, storeID uuid.UUID,
	) ([]repositories.Category, error)
	SetStoreCategoriesFn func(
		ctx context.Context, userID string, storeID uuid.UUID, categoryIDs []uuid.UUID,
	) error
	ListItemNamesFn func(
		ctx context.Context, userID string,
	) ([]repositories.ItemName, error)
	ListItemCategoriesFn func(
		ctx context.Context, userID string,
	) ([]repositories.ItemCategory, error)
	SetItemCategoryFn func(
		ctx context.Context, userID, name string, categoryID uuid.UUID,
	) error
}

func (m *ShoppingRepoMock) CheckPlanAccess(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
) error {
	return m.CheckPlanAccessFn(ctx, planID, userID)
}

func (m *ShoppingRepoMock) GetCustomItems(
	ctx context.Context,
	userID string,
) ([]repositories.ShoppingItem, error) {
	return m.GetCustomItemsFn(ctx, userID)
}

func (m *ShoppingRepoMock) AddCustomItem(
	ctx context.Context,
	userID, name, unit string,
	amount float64,
) (repositories.ShoppingItem, error) {
	return m.AddCustomItemFn(ctx, userID, name, unit, amount)
}

func (m *ShoppingRepoMock) DeleteCustomItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) error {
	return m.DeleteCustomItemFn(ctx, userID, itemID)
}

func (m *ShoppingRepoMock) GetMealPlanExportItems(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
	pastSlots []string,
) ([]repositories.ShoppingItem, error) {
	return m.GetMealPlanExportItemsFn(ctx, planID, start, end, pastSlots)
}

func (m *ShoppingRepoMock) ListCategories(
	ctx context.Context,
	userID string,
) ([]repositories.Category, error) {
	return m.ListCategoriesFn(ctx, userID)
}

func (m *ShoppingRepoMock) CreateCategory(
	ctx context.Context,
	userID, name string,
) (repositories.Category, error) {
	return m.CreateCategoryFn(ctx, userID, name)
}

func (m *ShoppingRepoMock) RenameCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) (repositories.Category, error) {
	return m.RenameCategoryFn(ctx, userID, id, name)
}

func (m *ShoppingRepoMock) DeleteCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return m.DeleteCategoryFn(ctx, userID, id)
}

func (m *ShoppingRepoMock) ListStores(
	ctx context.Context,
	userID string,
) ([]repositories.Store, error) {
	return m.ListStoresFn(ctx, userID)
}

func (m *ShoppingRepoMock) CreateStore(
	ctx context.Context,
	userID, name string,
) (repositories.Store, error) {
	return m.CreateStoreFn(ctx, userID, name)
}

func (m *ShoppingRepoMock) RenameStore(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) (repositories.Store, error) {
	return m.RenameStoreFn(ctx, userID, id, name)
}

func (m *ShoppingRepoMock) DeleteStore(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return m.DeleteStoreFn(ctx, userID, id)
}

func (m *ShoppingRepoMock) GetStoreCategories(
	ctx context.Context,
	userID string,
	storeID uuid.UUID,
) ([]repositories.Category, error) {
	return m.GetStoreCategoriesFn(ctx, userID, storeID)
}

func (m *ShoppingRepoMock) SetStoreCategories(
	ctx context.Context,
	userID string,
	storeID uuid.UUID,
	categoryIDs []uuid.UUID,
) error {
	return m.SetStoreCategoriesFn(ctx, userID, storeID, categoryIDs)
}

func (m *ShoppingRepoMock) ListItemNames(
	ctx context.Context,
	userID string,
) ([]repositories.ItemName, error) {
	return m.ListItemNamesFn(ctx, userID)
}

func (m *ShoppingRepoMock) ListItemCategories(
	ctx context.Context,
	userID string,
) ([]repositories.ItemCategory, error) {
	return m.ListItemCategoriesFn(ctx, userID)
}

func (m *ShoppingRepoMock) SetItemCategory(
	ctx context.Context,
	userID, name string,
	categoryID uuid.UUID,
) error {
	return m.SetItemCategoryFn(ctx, userID, name, categoryID)
}
