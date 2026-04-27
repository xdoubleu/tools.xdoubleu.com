package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/recipes/internal/models"
	"tools.xdoubleu.com/apps/recipes/internal/repositories"
	"tools.xdoubleu.com/internal/auth"
)

type PlanService struct {
	repo *repositories.PlansRepository
	auth auth.Service
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

	meals, err := s.repo.GetMealsWithRecipes(ctx, id)
	if err != nil {
		return nil, err
	}
	plan.Meals = meals
	return plan, nil
}

func (s *PlanService) GetByICalToken(
	ctx context.Context,
	token uuid.UUID,
) (*models.Plan, error) {
	plan, err := s.repo.GetByICalToken(ctx, token)
	if err != nil {
		return nil, err
	}

	meals, err := s.repo.GetMealsWithRecipes(ctx, plan.ID)
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
		return &HTTPError{
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
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "Only the owner can delete this plan",
		}
	}
	return s.repo.Delete(ctx, id, userID)
}

func (s *PlanService) AddMeal(
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
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not have edit access to this plan",
		}
	}
	meal.PlanID = planID
	_, err = s.repo.AddMeal(ctx, meal)
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
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "You do not have edit access to this plan",
		}
	}
	return s.repo.DeleteMeal(ctx, mealID, planID)
}

func (s *PlanService) ShareByEmail(
	ctx context.Context,
	planID uuid.UUID,
	ownerID string,
	email string,
	canEdit bool,
) error {
	existing, err := s.repo.GetByID(ctx, planID, ownerID)
	if err != nil {
		return err
	}
	if existing.OwnerUserID != ownerID {
		return &HTTPError{
			Status:  http.StatusForbidden,
			Message: "Only the owner can share this plan",
		}
	}

	users, err := s.auth.GetAllUsers()
	if err != nil {
		return err
	}

	for _, u := range users {
		if u.Email == email {
			return s.repo.SharePlan(ctx, planID, u.ID, canEdit)
		}
	}

	return &HTTPError{
		Status:  http.StatusNotFound,
		Message: "No user found with that email address",
	}
}
