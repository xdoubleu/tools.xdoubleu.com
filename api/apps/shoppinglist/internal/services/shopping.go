package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type Services struct {
	Auth     auth.Service
	Shopping *ShoppingService
}

func New(repo *repositories.ShoppingRepository, authService auth.Service) *Services {
	return &Services{
		Auth:     authService,
		Shopping: &ShoppingService{repo: repo},
	}
}

type ShoppingService struct {
	repo *repositories.ShoppingRepository
}

func (s *ShoppingService) GetList(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	start, end time.Time,
) ([]repositories.ShoppingItem, error) {
	if err := s.repo.CheckPlanAccess(ctx, planID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetShoppingList(ctx, planID, start, end)
}
