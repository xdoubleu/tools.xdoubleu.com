package services

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
)

func (s *ShoppingService) ListCategories(
	ctx context.Context,
	userID string,
) ([]repositories.Category, error) {
	return s.repo.ListCategories(ctx, userID)
}

func (s *ShoppingService) CreateCategory(
	ctx context.Context,
	userID, name string,
) (repositories.Category, error) {
	return s.repo.CreateCategory(ctx, userID, name)
}

func (s *ShoppingService) RenameCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) (repositories.Category, error) {
	return s.repo.RenameCategory(ctx, userID, id, name)
}

func (s *ShoppingService) DeleteCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	return s.repo.DeleteCategory(ctx, userID, id)
}

func (s *ShoppingService) ListItemNames(
	ctx context.Context,
	userID string,
) ([]repositories.ItemName, error) {
	return s.repo.ListItemNames(ctx, userID)
}

func (s *ShoppingService) ListItemCategories(
	ctx context.Context,
	userID string,
) ([]repositories.ItemCategory, error) {
	return s.repo.ListItemCategories(ctx, userID)
}

func (s *ShoppingService) SetItemCategory(
	ctx context.Context,
	userID, name string,
	categoryID uuid.UUID,
) error {
	return s.repo.SetItemCategory(ctx, userID, name, categoryID)
}
