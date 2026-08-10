package services

import (
	"context"

	"github.com/google/uuid"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

// settingsRepo is the storage surface SettingsService (and TaskService's URL
// pattern/label lookups) need. It is satisfied by
// repositories.SettingsRepository and by fakes in unit tests.
type settingsRepo interface {
	GetLabelPresets(
		ctx context.Context,
		userID string,
		workspaceID *uuid.UUID,
	) (*models.LabelPresets, error)
	CreateLabelPreset(
		ctx context.Context,
		userID string,
		category string,
		value string,
		workspaceID *uuid.UUID,
	) error
	DeleteLabelPreset(
		ctx context.Context,
		userID string,
		category string,
		value string,
		workspaceID *uuid.UUID,
	) error
	UpdateLabelPresetColor(
		ctx context.Context,
		userID string,
		category string,
		value string,
		workspaceID *uuid.UUID,
		color string,
	) error

	GetURLPatterns(
		ctx context.Context,
		userID string,
		workspaceID *uuid.UUID,
	) ([]models.URLPattern, error)
	CreateURLPattern(ctx context.Context, p models.URLPattern) error
	DeleteURLPattern(ctx context.Context, id uuid.UUID, userID string) error

	GetArchiveSettings(
		ctx context.Context,
		userID string,
	) (*models.ArchiveSettings, error)
	UpsertArchiveSettings(ctx context.Context, s models.ArchiveSettings) error

	GetUserSettings(ctx context.Context, userID string) (*models.UserSettings, error)
	SetActiveWorkspace(ctx context.Context, userID string, workspaceID *uuid.UUID) error
	UpdateHideShortcutHints(ctx context.Context, userID string, hide bool) error
}

// sectionsLister, policiesLister, and workspacesLister are the narrow
// surfaces GetSettings needs from its sibling services — just enough to
// fake the aggregation without a database. *SectionsService, *PoliciesService,
// and *WorkspacesService already satisfy these via their own List methods.
type sectionsLister interface {
	List(
		ctx context.Context,
		userID string,
		workspaceID *uuid.UUID,
	) ([]models.Section, error)
}

type policiesLister interface {
	List(
		ctx context.Context,
		userID string,
		workspaceID *uuid.UUID,
	) ([]models.Policy, error)
}

type workspacesLister interface {
	List(ctx context.Context, userID string) ([]models.Workspace, error)
}
