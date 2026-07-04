package services

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/dtos"
	"tools.xdoubleu.com/apps/todos/internal/models"
	"tools.xdoubleu.com/apps/todos/internal/repositories"
	"tools.xdoubleu.com/internal/app"
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

func (s *SettingsService) CreateLabelPreset(
	ctx context.Context,
	userID string,
	dto dtos.CreateLabelPresetDto,
	workspaceID *uuid.UUID,
) error {
	if dto.Value == "" {
		return &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Label value cannot be empty",
		}
	}
	if dto.Category != models.LabelCategory {
		return &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "Invalid label category",
		}
	}
	return s.settings.CreateLabelPreset(
		ctx,
		userID,
		dto.Category,
		dto.Value,
		workspaceID,
	)
}

func (s *SettingsService) DeleteLabelPreset(
	ctx context.Context,
	userID string,
	category string,
	value string,
	workspaceID *uuid.UUID,
) error {
	return s.settings.DeleteLabelPreset(ctx, userID, category, value, workspaceID)
}

func (s *SettingsService) GetURLPatterns(
	ctx context.Context,
	userID string,
	workspaceID *uuid.UUID,
) ([]models.URLPattern, error) {
	return s.settings.GetURLPatterns(ctx, userID, workspaceID)
}

func (s *SettingsService) CreateURLPattern(
	ctx context.Context,
	userID string,
	dto dtos.CreateURLPatternDto,
	workspaceID *uuid.UUID,
) error {
	if dto.URLPrefix == "" || dto.PlatformName == "" {
		return &app.HTTPError{
			Status:  http.StatusBadRequest,
			Message: "URL prefix and platform name are required",
		}
	}
	//nolint:exhaustruct // ID and SortOrder set by DB
	return s.settings.CreateURLPattern(ctx, models.URLPattern{
		UserID:       userID,
		URLPrefix:    dto.URLPrefix,
		PlatformName: dto.PlatformName,
		Label:        dto.Label,
		Shortcut:     strings.ToUpper(strings.TrimSpace(dto.Shortcut)),
		WorkspaceID:  workspaceID,
	})
}

func (s *SettingsService) DeleteURLPattern(
	ctx context.Context,
	id uuid.UUID,
	userID string,
) error {
	return s.settings.DeleteURLPattern(ctx, id, userID)
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

func (s *SettingsService) UpdateLabelColor(
	ctx context.Context,
	userID string,
	category string,
	value string,
	workspaceID *uuid.UUID,
	color string,
) error {
	return s.settings.UpdateLabelPresetColor(
		ctx,
		userID,
		category,
		value,
		workspaceID,
		color,
	)
}

func (s *SettingsService) UpdateHideShortcutHints(
	ctx context.Context,
	userID string,
	hide bool,
) error {
	return s.settings.UpdateHideShortcutHints(ctx, userID, hide)
}
