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
	GetCustomItems(
		ctx context.Context,
		userID string,
	) ([]repositories.ShoppingItem, error)
	AddCustomItem(
		ctx context.Context,
		userID, name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error)
	DeleteCustomItem(ctx context.Context, userID string, itemID uuid.UUID) error
	GetMealPlanExportItems(
		ctx context.Context,
		planID uuid.UUID,
		start, end time.Time,
	) ([]repositories.DayItems, error)
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

func (s *ShoppingService) GetCustomList(
	ctx context.Context,
	userID string,
) ([]repositories.ShoppingItem, error) {
	return s.repo.GetCustomItems(ctx, userID)
}

func (s *ShoppingService) AddItem(
	ctx context.Context,
	userID, name, unit string,
	amount float64,
) (repositories.ShoppingItem, error) {
	return s.repo.AddCustomItem(ctx, userID, name, unit, amount)
}

func (s *ShoppingService) DeleteItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) error {
	return s.repo.DeleteCustomItem(ctx, userID, itemID)
}

func (s *ShoppingService) GetMealPlanExportItems(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	start, end time.Time,
) ([]repositories.DayItems, error) {
	if err := s.repo.CheckPlanAccess(ctx, planID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetMealPlanExportItems(ctx, planID, start, end)
}
