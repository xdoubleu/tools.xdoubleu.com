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
	GetShoppingListFn func(
		ctx context.Context,
		planID uuid.UUID,
		start, end time.Time,
	) ([]repositories.ShoppingItem, error)
	AddCustomItemFn func(
		ctx context.Context,
		planID uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error)
	DeleteCustomItemFn func(ctx context.Context, planID, itemID uuid.UUID) error
}

func (m *ShoppingRepoMock) CheckPlanAccess(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
) error {
	return m.CheckPlanAccessFn(ctx, planID, userID)
}

func (m *ShoppingRepoMock) GetShoppingList(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
) ([]repositories.ShoppingItem, error) {
	return m.GetShoppingListFn(ctx, planID, start, end)
}

func (m *ShoppingRepoMock) AddCustomItem(
	ctx context.Context,
	planID uuid.UUID,
	name, unit string,
	amount float64,
) (repositories.ShoppingItem, error) {
	return m.AddCustomItemFn(ctx, planID, name, unit, amount)
}

func (m *ShoppingRepoMock) DeleteCustomItem(
	ctx context.Context,
	planID, itemID uuid.UUID,
) error {
	return m.DeleteCustomItemFn(ctx, planID, itemID)
}
