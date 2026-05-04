package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

type PoliciesService struct {
	policies *repositories.PoliciesRepository
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
		return nil, &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Policy text cannot be empty",
		}
	}
	if reappearAfterHours < 0 {
		return nil, &HTTPError{
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

func (s *PoliciesService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.policies.Delete(ctx, id, userID)
}
