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
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListCategories(ctx, familyID)
}

func (s *ShoppingService) CreateCategory(
	ctx context.Context,
	userID, name string,
) (repositories.Category, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return repositories.Category{}, err
	}
	return s.repo.CreateCategory(ctx, familyID, name)
}

func (s *ShoppingService) RenameCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
	name string,
) (repositories.Category, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return repositories.Category{}, err
	}
	return s.repo.RenameCategory(ctx, familyID, id, name)
}

func (s *ShoppingService) DeleteCategory(
	ctx context.Context,
	userID string,
	id uuid.UUID,
) error {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.DeleteCategory(ctx, familyID, id)
}

func (s *ShoppingService) ListItemNames(
	ctx context.Context,
	userID string,
) ([]repositories.ItemName, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListItemNames(ctx, familyID)
}

func (s *ShoppingService) ListItemCategories(
	ctx context.Context,
	userID string,
) ([]repositories.ItemCategory, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListItemCategories(ctx, familyID)
}

func (s *ShoppingService) SetItemCategory(
	ctx context.Context,
	userID, name string,
	categoryID uuid.UUID,
) error {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.SetItemCategory(ctx, familyID, name, categoryID)
}

func (s *ShoppingService) SetItemExcluded(
	ctx context.Context,
	userID, name string,
	excluded bool,
) error {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.SetItemExcluded(ctx, familyID, name, excluded)
}
