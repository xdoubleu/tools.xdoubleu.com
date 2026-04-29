package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

type WorkspacesService struct {
	workspaces *repositories.WorkspacesRepository
}

func (s *WorkspacesService) List(
	ctx context.Context,
	userID string,
) ([]models.Workspace, error) {
	return s.workspaces.ListByUser(ctx, userID)
}

func (s *WorkspacesService) Create(
	ctx context.Context,
	userID string,
	name string,
) (*models.Workspace, error) {
	if name == "" {
		return nil, &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Workspace name cannot be empty",
		}
	}
	//nolint:exhaustruct // ID, CreatedAt set by DB
	return s.workspaces.Create(ctx, models.Workspace{
		OwnerUserID: userID,
		Name:        name,
	})
}

func (s *WorkspacesService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.workspaces.Delete(ctx, id, userID)
}
