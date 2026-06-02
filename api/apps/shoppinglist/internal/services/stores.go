package services

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
)

func (s *ShoppingService) ListStores(
	ctx context.Context,
	userID string,
) ([]repositories.Store, error) {
	return s.repo.ListStores(ctx, userID)
}

func (s *ShoppingService) CreateStore(
	ctx context.Context,
	userID, name string,
) (repositories.Store, error) {
	return s.repo.CreateStore(ctx, userID, name)
}

func (s *ShoppingService) RenameStore(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) (repositories.Store, error) {
	return s.repo.RenameStore(ctx, userID, id, name)
}

func (s *ShoppingService) DeleteStore(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return s.repo.DeleteStore(ctx, userID, id)
}

func (s *ShoppingService) GetStoreCategories(
	ctx context.Context,
	userID string,
	storeID uuid.UUID,
) ([]repositories.Category, error) {
	return s.repo.GetStoreCategories(ctx, userID, storeID)
}

func (s *ShoppingService) SetStoreCategories(
	ctx context.Context,
	userID string,
	storeID uuid.UUID,
	categoryIDs []uuid.UUID,
) error {
	return s.repo.SetStoreCategories(ctx, userID, storeID, categoryIDs)
}
