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
	) ([]repositories.DayItems, error)
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
) ([]repositories.DayItems, error) {
	return m.GetMealPlanExportItemsFn(ctx, planID, start, end)
}
