package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type shoppingRepo interface {
	CheckPlanAccess(ctx context.Context, planID uuid.UUID, userID string) error
	GetShoppingList(
		ctx context.Context,
		planID uuid.UUID,
		start, end time.Time,
	) ([]repositories.ShoppingItem, error)
	AddCustomItem(
		ctx context.Context,
		planID uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error)
	DeleteCustomItem(ctx context.Context, planID, itemID uuid.UUID) error
}

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
	repo shoppingRepo
}

// NewShoppingService constructs a ShoppingService from any shoppingRepo implementation,
// allowing injection of mocks in tests.
func NewShoppingService(repo shoppingRepo) *ShoppingService {
	return &ShoppingService{repo: repo}
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

func (s *ShoppingService) AddItem(
	ctx context.Context,
	planID uuid.UUID,
	userID, name, unit string,
	amount float64,
) (repositories.ShoppingItem, error) {
	if err := s.repo.CheckPlanAccess(ctx, planID, userID); err != nil {
		return repositories.ShoppingItem{}, err
	}
	return s.repo.AddCustomItem(ctx, planID, name, unit, amount)
}

func (s *ShoppingService) DeleteItem(
	ctx context.Context,
	planID, itemID uuid.UUID,
	userID string,
) error {
	if err := s.repo.CheckPlanAccess(ctx, planID, userID); err != nil {
		return err
	}
	return s.repo.DeleteCustomItem(ctx, planID, itemID)
}
