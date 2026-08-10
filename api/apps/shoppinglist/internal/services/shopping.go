package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/shoppinglist/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

const (
	daysPerWeek = 7
	hoursPerDay = 24

	// Slot end hours (UTC) — match the iCal DTEND times in mealplans/ical.go.
	slotBreakfastEnd = 9
	slotNoonEnd      = 13
	slotEveningEnd   = 20

	slotBreakfast = "breakfast"
	slotNoon      = "noon"
	slotEvening   = "evening"
)

// ExportWindow returns the day-truncated start of the meal-plan export
// window plus which of today's slots (breakfast/noon/evening) have already
// passed, and the window's end: 7 days out normally, 8 when a slot has
// already passed today so that slot's next occurrence (next week) is still
// included.
func ExportWindow(now time.Time) (time.Time, time.Time, []string) {
	start := now.Truncate(hoursPerDay * time.Hour)
	var pastSlots []string
	if now.Hour() >= slotBreakfastEnd {
		pastSlots = append(pastSlots, slotBreakfast)
	}
	if now.Hour() >= slotNoonEnd {
		pastSlots = append(pastSlots, slotNoon)
	}
	if now.Hour() >= slotEveningEnd {
		pastSlots = append(pastSlots, slotEvening)
	}

	endOffset := daysPerWeek - 1
	if len(pastSlots) > 0 {
		endOffset = daysPerWeek
	}
	end := start.AddDate(0, 0, endOffset)

	return start, end, pastSlots
}

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
	UpdateCustomItem(
		ctx context.Context,
		userID string,
		itemID uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error)
	DeleteCustomItem(ctx context.Context, userID string, itemID uuid.UUID) error
	GetMealPlanExportItems(
		ctx context.Context,
		planID uuid.UUID,
		start, end time.Time,
		pastSlots []string,
		excludedGroups []string,
	) ([]repositories.ShoppingItem, error)
	GetPlanIngredientGroups(
		ctx context.Context,
		planID uuid.UUID,
		start, end time.Time,
		pastSlots []string,
	) ([]repositories.PlanIngredientGroup, error)

	ListCategories(ctx context.Context, userID string) ([]repositories.Category, error)
	CreateCategory(
		ctx context.Context,
		userID, name string,
	) (repositories.Category, error)
	RenameCategory(
		ctx context.Context,
		userID string,
		id uuid.UUID,
		name string,
	) (repositories.Category, error)
	DeleteCategory(ctx context.Context, userID string, id uuid.UUID) error

	ListStores(ctx context.Context, userID string) ([]repositories.Store, error)
	CreateStore(ctx context.Context, userID, name string) (repositories.Store, error)
	RenameStore(
		ctx context.Context,
		userID string,
		id uuid.UUID,
		name string,
	) (repositories.Store, error)
	DeleteStore(ctx context.Context, userID string, id uuid.UUID) error
	GetStoreCategories(
		ctx context.Context,
		userID string,
		storeID uuid.UUID,
	) ([]repositories.Category, error)
	SetStoreCategories(
		ctx context.Context,
		userID string,
		storeID uuid.UUID,
		categoryIDs []uuid.UUID,
	) error

	ListItemNames(ctx context.Context, userID string) ([]repositories.ItemName, error)
	ListItemCategories(
		ctx context.Context,
		userID string,
	) ([]repositories.ItemCategory, error)
	SetItemCategory(
		ctx context.Context,
		userID, name string,
		categoryID uuid.UUID,
	) error
	SetItemExcluded(
		ctx context.Context,
		userID, name string,
		excluded bool,
	) error
}

type Services struct {
	Auth     auth.Service
	Shopping *ShoppingService
	Sharing  *SharingService
}

func New(repo *repositories.ShoppingRepository, authService auth.Service) *Services {
	return &Services{
		Auth:     authService,
		Shopping: &ShoppingService{repo: repo},
		Sharing:  &SharingService{repo: repo},
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

func (s *ShoppingService) UpdateItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	name, unit string,
	amount float64,
) (repositories.ShoppingItem, error) {
	return s.repo.UpdateCustomItem(ctx, userID, itemID, name, unit, amount)
}

func (s *ShoppingService) DeleteItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) error {
	return s.repo.DeleteCustomItem(ctx, userID, itemID)
}

// GetMealPlanExportItems returns only the aggregated meal-plan ingredients for
// the plan. Custom items are intentionally not included here: the frontend
// fetches them separately and merges them once. Because the export hook calls
// this per meal plan, appending custom items here would duplicate them once per
// plan (plus once more from the separate custom-list fetch).
func (s *ShoppingService) GetMealPlanExportItems(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	start, end time.Time,
	pastSlots []string,
	excludedGroups []string,
) ([]repositories.ShoppingItem, error) {
	if err := s.repo.CheckPlanAccess(ctx, planID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetMealPlanExportItems(
		ctx, planID, start, end, pastSlots, excludedGroups,
	)
}

func (s *ShoppingService) GetPlanIngredientGroups(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	start, end time.Time,
	pastSlots []string,
) ([]repositories.PlanIngredientGroup, error) {
	if err := s.repo.CheckPlanAccess(ctx, planID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetPlanIngredientGroups(ctx, planID, start, end, pastSlots)
}
