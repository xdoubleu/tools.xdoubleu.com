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

func baseMock() *mocks.ShoppingRepoMock {
	return &mocks.ShoppingRepoMock{
		CheckPlanAccessFn:        nil,
		GetCustomItemsFn:         nil,
		AddCustomItemFn:          nil,
		DeleteCustomItemFn:       nil,
		GetMealPlanExportItemsFn: nil,
	}
}

func accessGrantedMock() *mocks.ShoppingRepoMock {
	m := baseMock()
	m.CheckPlanAccessFn = func(_ context.Context, _ uuid.UUID, _ string) error {
		return nil
	}
	return m
}

func accessDeniedMock() *mocks.ShoppingRepoMock {
	m := baseMock()
	m.CheckPlanAccessFn = func(_ context.Context, _ uuid.UUID, _ string) error {
		return errNotFound
	}
	return m
}

func TestGetCustomList_ReturnsItems(t *testing.T) {
	want := []repositories.ShoppingItem{
		{ID: "id-1", Name: "milk", Unit: "L", Amount: 1},
		{ID: "id-2", Name: "eggs", Unit: "", Amount: 6},
	}
	m := baseMock()
	m.GetCustomItemsFn = func(
		_ context.Context, userID string,
	) ([]repositories.ShoppingItem, error) {
		assert.Equal(t, "user1", userID)
		return want, nil
	}

	svc := services.NewShoppingService(m)
	got, err := svc.GetCustomList(context.Background(), "user1")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetCustomList_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.GetCustomItemsFn = func(
		_ context.Context, _ string,
	) ([]repositories.ShoppingItem, error) {
		return nil, repoErr
	}

	svc := services.NewShoppingService(m)
	_, err := svc.GetCustomList(context.Background(), "user1")
	assert.ErrorIs(t, err, repoErr)
}

func TestAddItem_Success(t *testing.T) {
	want := repositories.ShoppingItem{
		ID:     uuid.NewString(),
		Name:   "milk",
		Unit:   "L",
		Amount: 1,
	}
	m := baseMock()
	m.AddCustomItemFn = func(
		_ context.Context,
		userID, name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error) {
		assert.Equal(t, "user1", userID)
		assert.Equal(t, "milk", name)
		assert.Equal(t, "L", unit)
		assert.InDelta(t, 1.0, amount, 1e-9)
		return want, nil
	}

	svc := services.NewShoppingService(m)
	got, err := svc.AddItem(context.Background(), "user1", "milk", "L", 1)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAddItem_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.AddCustomItemFn = func(
		_ context.Context, _, _, _ string, _ float64,
	) (repositories.ShoppingItem, error) {
		return repositories.ShoppingItem{}, repoErr
	}

	svc := services.NewShoppingService(m)
	_, err := svc.AddItem(context.Background(), "user1", "milk", "L", 1)
	assert.ErrorIs(t, err, repoErr)
}

func TestDeleteItem_Success(t *testing.T) {
	itemID := uuid.New()
	m := baseMock()
	m.DeleteCustomItemFn = func(_ context.Context, userID string, iID uuid.UUID) error {
		assert.Equal(t, "user1", userID)
		assert.Equal(t, itemID, iID)
		return nil
	}

	svc := services.NewShoppingService(m)
	err := svc.DeleteItem(context.Background(), "user1", itemID)
	assert.NoError(t, err)
}

func TestDeleteItem_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := baseMock()
	m.DeleteCustomItemFn = func(_ context.Context, _ string, _ uuid.UUID) error {
		return repoErr
	}

	svc := services.NewShoppingService(m)
	err := svc.DeleteItem(context.Background(), "user1", uuid.New())
	assert.ErrorIs(t, err, repoErr)
}

func TestGetMealPlanExportItems_AccessDenied(t *testing.T) {
	svc := services.NewShoppingService(accessDeniedMock())
	start := time.Now().UTC()
	_, err := svc.GetMealPlanExportItems(
		context.Background(), uuid.New(), "user1", start, start.AddDate(0, 0, 6), []string{},
	)
	assert.ErrorIs(t, err, errNotFound)
}

func TestGetMealPlanExportItems_Success(t *testing.T) {
	planID := uuid.New()
	start := time.Now().UTC()
	end := start.AddDate(0, 0, 6)
	pastSlots := []string{"breakfast"}
	want := []repositories.ShoppingItem{
		{ID: "", Name: "flour", Unit: "g", Amount: 200},
	}
	m := accessGrantedMock()
	m.GetMealPlanExportItemsFn = func(
		_ context.Context, pID uuid.UUID, s, e time.Time, ps []string,
	) ([]repositories.ShoppingItem, error) {
		assert.Equal(t, planID, pID)
		assert.Equal(t, start, s)
		assert.Equal(t, end, e)
		assert.Equal(t, pastSlots, ps)
		return want, nil
	}

	svc := services.NewShoppingService(m)
	got, err := svc.GetMealPlanExportItems(
		context.Background(),
		planID,
		"user1",
		start,
		end,
		pastSlots,
	)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetMealPlanExportItems_RepoError(t *testing.T) {
	repoErr := errors.New("db error")
	m := accessGrantedMock()
	m.GetMealPlanExportItemsFn = func(
		_ context.Context, _ uuid.UUID, _, _ time.Time, _ []string,
	) ([]repositories.ShoppingItem, error) {
		return nil, repoErr
	}

	svc := services.NewShoppingService(m)
	start := time.Now().UTC()
	_, err := svc.GetMealPlanExportItems(
		context.Background(), uuid.New(), "user1", start, start.AddDate(0, 0, 6), []string{},
	)
	assert.ErrorIs(t, err, repoErr)
}
