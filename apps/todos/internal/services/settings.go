package services

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
)

type SettingsService struct {
	settings *repositories.SettingsRepository
}

func (s *SettingsService) GetLabelPresets(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) (*models.LabelPresets, error) {
	return s.settings.GetLabelPresets(ctx, userID, workspaceID)
}

func (s *SettingsService) AddLabelPreset(
	ctx context.Context,
	userID string,
	dto dtos.AddLabelPresetDto,
	workspaceID *uuid.UUID,
) error {
	if dto.Value == "" {
		return &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Label value cannot be empty",
		}
	}
	if dto.Category != models.LabelCategorySetup &&
		dto.Category != models.LabelCategoryType {
		return &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid label category",
		}
	}
	return s.settings.AddLabelPreset(ctx, userID, dto.Category, dto.Value, workspaceID)
}

func (s *SettingsService) RemoveLabelPreset(
	ctx context.Context,
	userID string,
	category string,
	value string,
	workspaceID *uuid.UUID,
) error {
	return s.settings.RemoveLabelPreset(ctx, userID, category, value, workspaceID)
}

func (s *SettingsService) GetURLPatterns(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) ([]models.URLPattern, error) {
	return s.settings.GetURLPatterns(ctx, userID, workspaceID)
}

func (s *SettingsService) AddURLPattern(
	ctx context.Context,
	userID string,
	dto dtos.AddURLPatternDto,
	workspaceID *uuid.UUID,
) error {
	if dto.URLPrefix == "" || dto.PlatformName == "" {
		return &HTTPError{
			Status:  http.StatusBadRequest,
			Message: "URL prefix and platform name are required",
		}
	}
	//nolint:exhaustruct // ID and SortOrder set by DB
	return s.settings.AddURLPattern(ctx, models.URLPattern{
		UserID:       userID,
		URLPrefix:    dto.URLPrefix,
		PlatformName: dto.PlatformName,
		TypeLabel:    dto.TypeLabel,
		WorkspaceID:  workspaceID,
	})
}

func (s *SettingsService) RemoveURLPattern(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.settings.RemoveURLPattern(ctx, id, userID)
}

func (s *SettingsService) GetArchiveSettings(
	ctx context.Context,
	userID string,
) (*models.ArchiveSettings, error) {
	return s.settings.GetArchiveSettings(ctx, userID)
}

func (s *SettingsService) UpdateArchiveSettings(
	ctx context.Context,
	userID string,
	hours int,
) error {
	return s.settings.UpsertArchiveSettings(ctx, models.ArchiveSettings{
		UserID:            userID,
		ArchiveAfterHours: hours,
	})
}

func (s *SettingsService) GetUserSettings(
	ctx context.Context,
	userID string,
) (*models.UserSettings, error) {
	return s.settings.GetUserSettings(ctx, userID)
}

func (s *SettingsService) SetActiveWorkspace(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) error {
	return s.settings.SetActiveWorkspace(ctx, userID, workspaceID)
}
