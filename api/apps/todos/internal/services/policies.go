package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/internal/app"
)

// policiesRepo is the storage surface PoliciesService needs. It is
// satisfied by repositories.PoliciesRepository and by fakes in unit tests.
type policiesRepo interface {
	ListByUser(
		ctx context.Context,
		userID string,
		workspaceID *uuid.UUID,
	) ([]models.Policy, error)
	Create(ctx context.Context, p models.Policy) (*models.Policy, error)
	Update(
		ctx context.Context,
		id uuid.UUID,
		userID string,
		text string,
		reappearAfterHours int,
	) (*models.Policy, error)
	Delete(ctx context.Context, id uuid.UUID, userID string) error
}

type PoliciesService struct {
	policies policiesRepo
}

func (s *PoliciesService) List(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) ([]models.Policy, error) {
	return s.policies.ListByUser(ctx, userID, workspaceID)
}

func (s *PoliciesService) Create(
	ctx context.Context,
	userID string,
	text string,
	reappearAfterHours int,
	workspaceID *uuid.UUID,
) (*models.Policy, error) {
	if text == "" {
		return nil, &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Policy text cannot be empty",
		}
	}
	if reappearAfterHours < 0 {
		return nil, &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Reappear hours must be non-negative",
		}
	}
	//nolint:exhaustruct // ID, SortOrder, CreatedAt set by DB
	return s.policies.Create(ctx, models.Policy{
		OwnerUserID:        userID,
		Text:               text,
		ReappearAfterHours: reappearAfterHours,
		WorkspaceID:        workspaceID,
	})
}

func (s *PoliciesService) Update(
	ctx context.Context,
	id uuid.UUID,
	userID string,
	text string,
	reappearAfterHours int,
) (*models.Policy, error) {
	if text == "" {
		return nil, &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Policy text cannot be empty",
		}
	}
	if reappearAfterHours < 0 {
		return nil, &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Reappear hours must be non-negative",
		}
	}
	return s.policies.Update(ctx, id, userID, text, reappearAfterHours)
}

func (s *PoliciesService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.policies.Delete(ctx, id, userID)
}
