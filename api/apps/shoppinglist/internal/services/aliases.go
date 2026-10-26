package services

import (
	"context"
	"strings"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
)

// SetItemAlias exports name as canonical; an empty canonical unmerges name.
func (s *ShoppingService) SetItemAlias(
	ctx context.Context,
	userID, name, canonical string,
) error {
	alias := normalizeItemName(name)
	canonical = normalizeItemName(canonical)
	if alias == canonical {
		return repositories.ErrSelfAlias
	}

	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	if canonical == "" {
		return s.repo.DeleteItemAlias(ctx, familyID, alias)
	}
	return s.repo.SetItemAlias(ctx, familyID, alias, canonical)
}

func (s *ShoppingService) ListIngredientUnits(
	ctx context.Context,
	userID string,
) ([]repositories.IngredientUnit, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListIngredientUnits(ctx, familyID)
}

// normalizeItemName matches the SQL LOWER(TRIM(name)) catalog key.
func normalizeItemName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
