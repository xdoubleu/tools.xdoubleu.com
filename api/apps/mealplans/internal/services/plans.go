package services

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/mealplans/internal/models"
	"tools.xdoubleu.com/internal/app"
)

const errNoEditAccess = "You do not have edit access to this plan"

// plansStore is the storage surface PlanService needs. It is satisfied by
// repositories.PlansRepository and by fakes in unit tests, so the ownership
// and edit-access rules can be tested without a database.
type plansStore interface {
	ListForUser(ctx context.Context, userID string) ([]models.Plan, error)
	GetByID(ctx context.Context, id uuid.UUID, userID string) (*models.Plan, error)
	GetSharedWith(
		ctx context.Context,
		id uuid.UUID,
		userID string,
	) ([]models.PlanSharedUser, error)
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
	Delete(ctx context.Context, id uuid.UUID, userID string) error
	CreateMeal(ctx context.Context, meal models.PlanMeal) (*models.PlanMeal, error)
	DeleteMeal(ctx context.Context, mealID, planID uuid.UUID) error
	MoveMeal(
		ctx context.Context,
		mealID, planID uuid.UUID,
		newDate time.Time,
		newSlot string,
	) error
	UnshareUser(ctx context.Context, planID uuid.UUID, targetUserID string) error
	SharePlan(
		ctx context.Context,
		planID uuid.UUID,
		contactUserID string,
		canEdit bool,
	) error
}

type PlanService struct {
	repo plansStore
}

func (s *PlanService) List(
	ctx context.Context,
	userID string,
) ([]models.Plan, error) {
	return s.repo.ListForUser(ctx, userID)
}

func (s *PlanService) Get(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) (*models.Plan, error) {
	plan, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if plan.OwnerUserID == userID {
		plan.SharedWith, err = s.repo.GetSharedWith(ctx, id, userID)
		if err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func (s *PlanService) GetMeals(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	start, end time.Time,
) ([]models.PlanMeal, error) {
	if _, err := s.repo.GetByID(ctx, planID, userID); err != nil {
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
	if _, err := s.repo.GetByID(ctx, planID, userID); err != nil {
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
	plan.OwnerUserID = userID
	return s.repo.Create(ctx, plan)
}

func (s *PlanService) Update(
	ctx context.Context,
	userID string,
	plan models.Plan,
) error {
	existing, err := s.repo.GetByID(ctx, plan.ID, userID)
	if err != nil {
		return err
	}
	if existing.OwnerUserID != userID {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: "Only the owner can edit plan details",
		}
	}
	plan.OwnerUserID = userID
	return s.repo.Update(ctx, plan)
}

func (s *PlanService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	existing, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if existing.OwnerUserID != userID {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: "Only the owner can delete this plan",
		}
	}
	return s.repo.Delete(ctx, id, userID)
}

func (s *PlanService) CreateMeal(
	ctx context.Context,
	planID uuid.UUID,
	userID string,
	meal models.PlanMeal,
) error {
	plan, err := s.repo.GetByID(ctx, planID, userID)
	if err != nil {
		return err
	}
	if !plan.CanEdit {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: errNoEditAccess,
		}
	}
	meal.PlanID = planID
	_, err = s.repo.CreateMeal(ctx, meal)
	return err
}

func (s *PlanService) DeleteMeal(
	ctx context.Context,
	mealID uuid.UUID,
	planID uuid.UUID,
	userID string,
) error {
	plan, err := s.repo.GetByID(ctx, planID, userID)
	if err != nil {
		return err
	}
	if !plan.CanEdit {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: errNoEditAccess,
		}
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
	plan, err := s.repo.GetByID(ctx, planID, userID)
	if err != nil {
		return err
	}
	if !plan.CanEdit {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: errNoEditAccess,
		}
	}
	return s.repo.MoveMeal(ctx, mealID, planID, newDate, newSlot)
}

func (s *PlanService) Unshare(
	ctx context.Context,
	planID uuid.UUID,
	ownerID string,
	targetUserID string,
) error {
	existing, err := s.repo.GetByID(ctx, planID, ownerID)
	if err != nil {
		return err
	}
	if existing.OwnerUserID != ownerID {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: "Only the owner can modify sharing",
		}
	}
	return s.repo.UnshareUser(ctx, planID, targetUserID)
}

func (s *PlanService) Share(
	ctx context.Context,
	planID uuid.UUID,
	ownerID string,
	contactUserID string,
	canEdit bool,
) error {
	existing, err := s.repo.GetByID(ctx, planID, ownerID)
	if err != nil {
		return err
	}
	if existing.OwnerUserID != ownerID {
		return &app.HTTPError{
			Status:  http.StatusForbidden,
			Message: "Only the owner can share this plan",
		}
	}
	return s.repo.SharePlan(ctx, planID, contactUserID, canEdit)
}
