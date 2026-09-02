package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
)

const (
	daysPerWeek = 7
	hoursPerDay = 24
)

// WeekWindow returns the 7-day window that starts `offset` weeks from today
// (UTC, truncated to the day), used both to page through a plan's meals and
// to render its iCal feed.
func WeekWindow(offset int) (time.Time, time.Time) {
	today := time.Now().UTC().Truncate(hoursPerDay * time.Hour)
	start := today.AddDate(0, 0, daysPerWeek*offset)
	end := start.AddDate(0, 0, daysPerWeek-1)
	return start, end
}

// plansStore is the storage surface PlanService needs. It is satisfied by
// repositories.PlansRepository and by fakes in unit tests, so the
// family-scoping rules can be tested without a database.
type plansStore interface {
	ListForFamily(
		ctx context.Context, familyID uuid.UUID, limit, offset int32,
	) ([]models.Plan, bool, error)
	GetByID(ctx context.Context, id uuid.UUID, familyID uuid.UUID) (*models.Plan, error)
	GetMealsInWindow(
		ctx context.Context,
		planID uuid.UUID,
		start, end time.Time,
	) ([]models.PlanMeal, error)
	SuggestRecipes(
		ctx context.Context,
		planID uuid.UUID,
		mealDate time.Time,
		slot string,
		limit int,
	) ([]models.RecipeSuggestion, error)
	GetByICalToken(ctx context.Context, token uuid.UUID) (*models.Plan, error)
	Create(ctx context.Context, plan models.Plan) (*models.Plan, error)
	Update(ctx context.Context, plan models.Plan) error
	Delete(ctx context.Context, id uuid.UUID, familyID uuid.UUID) error
	CreateMeal(ctx context.Context, meal models.PlanMeal) (*models.PlanMeal, error)
	DeleteMeal(ctx context.Context, mealID, planID uuid.UUID) error
	MoveMeal(
		ctx context.Context,
		mealID, planID uuid.UUID,
		newDate time.Time,
		newSlot string,
	) error
}

// familyStore resolves which family a user belongs to, lazily creating a
// family-of-one the first time it's asked for (see internal/family).
type familyStore interface {
	EnsureFamily(ctx context.Context, userID string) (uuid.UUID, error)
}

type PlanService struct {
	repo   plansStore
	family familyStore
}

func (s *PlanService) List(
	ctx context.Context,
	userID string,
	limit int32,
	offset int32,
) ([]models.Plan, bool, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	return s.repo.ListForFamily(ctx, familyID, limit, offset)
}

func (s *PlanService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Plan, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id, familyID)
}

// GetWithWeek returns the plan along with its meals for the 7-day window
// `offset` weeks from today, and the resolved window bounds.
func (s *PlanService) GetWithWeek(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	offset int,
) (*models.Plan, time.Time, time.Time, error) {
	plan, err := s.Get(ctx, id, userID)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}

	windowStart, windowEnd := WeekWindow(offset)

	meals, err := s.GetMeals(ctx, id, userID, windowStart, windowEnd)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	plan.Meals = meals

	return plan, windowStart, windowEnd, nil
}

func (s *PlanService) GetMeals(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	start, end time.Time,
) ([]models.PlanMeal, error) {
	if _, err := s.Get(ctx, planID, userID); err != nil {
		return nil, err
	}
	return s.repo.GetMealsInWindow(ctx, planID, start, end)
}

// suggestRecipesLimit caps how many suggestions are returned per cell.
const suggestRecipesLimit = 5

func (s *PlanService) SuggestRecipes(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	mealDate time.Time,
	slot string,
) ([]models.RecipeSuggestion, error) {
	if _, err := s.Get(ctx, planID, userID); err != nil {
		return nil, err
	}
	return s.repo.SuggestRecipes(ctx, planID, mealDate, slot, suggestRecipesLimit)
}

func (s *PlanService) GetByICalToken(
	ctx context.Context,
	token uuid.UUID,
) (*models.Plan, error) {
	plan, err := s.repo.GetByICalToken(ctx, token)
	if err != nil {
		return nil, err
	}

	meals, err := s.repo.GetMealsInWindow(ctx, plan.ID, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	plan.Meals = meals
	return plan, nil
}

func (s *PlanService) Create(
	ctx context.Context,
	userID string,
	plan models.Plan,
) (*models.Plan, error) {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return nil, err
	}
	plan.OwnerUserID = userID
	plan.FamilyID = familyID
	return s.repo.Create(ctx, plan)
}

func (s *PlanService) Update(
	ctx context.Context,
	userID string,
	plan models.Plan,
) error {
	existing, err := s.Get(ctx, plan.ID, userID)
	if err != nil {
		return err
	}
	plan.FamilyID = existing.FamilyID
	return s.repo.Update(ctx, plan)
}

func (s *PlanService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	familyID, err := s.family.EnsureFamily(ctx, userID)
	if err != nil {
		return err
	}
	if _, err = s.repo.GetByID(ctx, id, familyID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id, familyID)
}

func (s *PlanService) CreateMeal(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	meal models.PlanMeal,
) error {
	if _, err := s.Get(ctx, planID, userID); err != nil {
		return err
	}
	meal.PlanID = planID
	_, err := s.repo.CreateMeal(ctx, meal)
	return err
}

func (s *PlanService) DeleteMeal(
	ctx context.Context,
	mealID uuid.UUID,
	planID uuid.UUID,
	userID string,
) error {
	if _, err := s.Get(ctx, planID, userID); err != nil {
		return err
	}
	return s.repo.DeleteMeal(ctx, mealID, planID)
}

func (s *PlanService) MoveMeal(
	ctx context.Context,
	mealID uuid.UUID,
	planID uuid.UUID,
	userID string,
	newDate time.Time,
	newSlot string,
) error {
	if _, err := s.Get(ctx, planID, userID); err != nil {
		return err
	}
	return s.repo.MoveMeal(ctx, mealID, planID, newDate, newSlot)
}
