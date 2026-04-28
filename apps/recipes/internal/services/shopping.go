package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/repositories"
)

type ShoppingService struct {
	repo *repositories.ShoppingRepository
}

func (s *ShoppingService) GetList(
	ctx context.Context,
	planID uuid.UUID,
	start, end time.Time,
) ([]models.ShoppingItem, error) {
	return s.repo.GetShoppingList(ctx, planID, start, end)
}
