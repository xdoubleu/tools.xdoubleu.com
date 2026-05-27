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

func accessDeniedMock() *mocks.ShoppingRepoMock {
	return &mocks.ShoppingRepoMock{
		CheckPlanAccessFn: func(
			_ context.Context,
			_ uuid.UUID,
			_ string,
		) error {
			return errNotFound
		},
		GetShoppingListFn:  nil,
		AddCustomItemFn:    nil,
		DeleteCustomItemFn: nil,
	}
}

func accessGrantedMock() *mocks.ShoppingRepoMock {
	return &mocks.ShoppingRepoMock{
		CheckPlanAccessFn: func(
			_ context.Context,
			_ uuid.UUID,
			_ string,
		) error {
			return nil
		},
		GetShoppingListFn:  nil,
		AddCustomItemFn:    nil,
		DeleteCustomItemFn: nil,
	}
}

func TestAddItem_AccessDenied(t *testing.T) {
	svc := services.NewShoppingService(accessDeniedMock())
	_, err := svc.AddItem(context.Background(), uuid.New(), "user1", "milk", "L", 1)
	assert.ErrorIs(t, err, errNotFound)
}

func TestAddItem_Success(t *testing.T) {
	planID := uuid.New()
	want := repositories.ShoppingItem{
		ID:     uuid.NewString(),
		Name:   "milk",
		Unit:   "L",
		Amount: 1,
	}
	repo := accessGrantedMock()
	repo.AddCustomItemFn = func(
		_ context.Context,
		_ uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error) {
		assert.Equal(t, "milk", name)
		assert.Equal(t, "L", unit)
		assert.InDelta(t, 1.0, amount, 1e-9)
		return want, nil
	}
	svc := services.NewShoppingService(repo)

	got, err := svc.AddItem(context.Background(), planID, "user1", "milk", "L", 1)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDeleteItem_AccessDenied(t *testing.T) {
	svc := services.NewShoppingService(accessDeniedMock())
	err := svc.DeleteItem(context.Background(), uuid.New(), uuid.New(), "user1")
	assert.ErrorIs(t, err, errNotFound)
}

func TestDeleteItem_Success(t *testing.T) {
	planID := uuid.New()
	itemID := uuid.New()
	repo := accessGrantedMock()
	repo.DeleteCustomItemFn = func(_ context.Context, pID, iID uuid.UUID) error {
		assert.Equal(t, planID, pID)
		assert.Equal(t, itemID, iID)
		return nil
	}
	svc := services.NewShoppingService(repo)

	err := svc.DeleteItem(context.Background(), planID, itemID, "user1")
	assert.NoError(t, err)
}

func TestGetList_ReturnsSeparateLists(t *testing.T) {
	planID := uuid.New()
	start := time.Now().UTC()
	end := start.AddDate(0, 0, 6)
	want := repositories.ShoppingLists{
		MealPlanItems: []repositories.ShoppingItem{
			{ID: "", Name: "flour", Unit: "g", Amount: 200},
		},
		CustomItems: []repositories.ShoppingItem{
			{ID: "custom-1", Name: "milk", Unit: "L", Amount: 1},
		},
	}
	repo := accessGrantedMock()
	repo.GetShoppingListFn = func(
		_ context.Context,
		_ uuid.UUID,
		_, _ time.Time,
	) (repositories.ShoppingLists, error) {
		return want, nil
	}
	svc := services.NewShoppingService(repo)

	got, err := svc.GetList(context.Background(), planID, "user1", start, end)
	require.NoError(t, err)
	assert.Equal(t, want.MealPlanItems, got.MealPlanItems)
	assert.Equal(t, want.CustomItems, got.CustomItems)
}

func TestDeleteItem_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	repo := accessGrantedMock()
	repo.DeleteCustomItemFn = func(_ context.Context, _, _ uuid.UUID) error {
		return repoErr
	}
	svc := services.NewShoppingService(repo)

	err := svc.DeleteItem(context.Background(), uuid.New(), uuid.New(), "user1")
	assert.ErrorIs(t, err, repoErr)
}
