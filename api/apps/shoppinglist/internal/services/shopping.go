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

// familyStore resolves which family a user belongs to, lazily creating a
// family-of-one the first time it's asked for (see internal/family).
type familyStore interface {
	EnsureFamily(ctx context.Context, userID string) (uuid.UUID, error)
}

type shoppingRepo interface {
	CheckPlanAccess(ctx context.Context, planID uuid.UUID, familyID uuid.UUID) error
	GetCustomItems(
		ctx context.Context,
		familyID uuid.UUID,
	) ([]repositories.ShoppingItem, error)
	AddCustomItem(
		ctx context.Context,
		familyID uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error)
	UpdateCustomItem(
		ctx context.Context,
		familyID uuid.UUID,
		itemID uuid.UUID,
		name, unit string,
		amount float64,
	) (repositories.ShoppingItem, error)
	DeleteCustomItem(ctx context.Context, familyID uuid.UUID, itemID uuid.UUID) error
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

	ListCategories(
		ctx context.Context,
		familyID uuid.UUID,
	) ([]repositories.Category, error)
	CreateCategory(
		ctx context.Context,
		familyID uuid.UUID,
		name string,
	) (repositories.Category, error)
	RenameCategory(
		ctx context.Context,
		familyID uuid.UUID,
		id uuid.UUID,
		name string,
	) (repositories.Category, error)
	DeleteCategory(ctx context.Context, familyID uuid.UUID, id uuid.UUID) error

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
		familyID uuid.UUID,
		storeID uuid.UUID,
		categoryIDs []uuid.UUID,
	) error

	ListItemNames(
		ctx context.Context,
		familyID uuid.UUID,
	) ([]repositories.ItemName, error)
	ListItemCategories(
		ctx context.Context,
		familyID uuid.UUID,
	) ([]repositories.ItemCategory, error)
	SetItemCategory(
		ctx context.Context,
		familyID uuid.UUID,
		name string,
		categoryID uuid.UUID,
	) error
	SetItemExcluded(
		ctx context.Context,
		familyID uuid.UUID,
		name string,
		excluded bool,
	) error
}

type Services struct {
	Auth     auth.Service
	Shopping *ShoppingService
}

func New(
	repo *repositories.ShoppingRepository,
	authService auth.Service,
	family familyStore,
) *Services {
	return &Services{
		Auth:     authService,
		Shopping: &ShoppingService{repo: repo, family: family},
	}
}

type ShoppingService struct {
	repo   shoppingRepo
	family familyStore
}

// NewShoppingService constructs a ShoppingService from any shoppingRepo
// implementation, allowing injection of mocks in tests.
func NewShoppingService(repo shoppingRepo, family familyStore) *ShoppingService {
	return &ShoppingService{repo: repo, family: family}
}

func (s *ShoppingService) GetCustomList(
	ctx context.Context,
	userID string,
) ([]repositories.ShoppingItem, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetCustomItems(ctx, familyID)
}

func (s *ShoppingService) AddItem(
	ctx context.Context,
	userID, name, unit string,
	amount float64,
) (repositories.ShoppingItem, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return repositories.ShoppingItem{}, err
	}
	return s.repo.AddCustomItem(ctx, familyID, name, unit, amount)
}

func (s *ShoppingService) UpdateItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
	name, unit string,
	amount float64,
) (repositories.ShoppingItem, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return repositories.ShoppingItem{}, err
	}
	return s.repo.UpdateCustomItem(ctx, familyID, itemID, name, unit, amount)
}

func (s *ShoppingService) DeleteItem(
	ctx context.Context,
	userID string,
	itemID uuid.UUID,
) error {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	return s.repo.DeleteCustomItem(ctx, familyID, itemID)
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
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err = s.repo.CheckPlanAccess(ctx, planID, familyID); err != nil {
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
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err = s.repo.CheckPlanAccess(ctx, planID, familyID); err != nil {
		return nil, err
	}
	return s.repo.GetPlanIngredientGroups(ctx, planID, start, end, pastSlots)
}
