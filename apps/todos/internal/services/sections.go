package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

type SectionsService struct {
	sections *repositories.SectionsRepository
}

func (s *SectionsService) List(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) ([]models.Section, error) {
	return s.sections.ListByUser(ctx, userID, workspaceID)
}

func (s *SectionsService) Create(
	ctx context.Context,
	userID string,
	dto dtos.AddSectionDto,
	workspaceID *uuid.UUID,
) (*models.Section, error) {
	if dto.Name == "" {
		return nil, &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Section name cannot be empty",
		}
	}
	//nolint:exhaustruct // ID, SortOrder, CreatedAt set by DB
	return s.sections.Create(ctx, models.Section{
		OwnerUserID: userID,
		Name:        dto.Name,
		WorkspaceID: workspaceID,
	})
}

func (s *SectionsService) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.sections.Delete(ctx, id, userID)
}
