package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

// fakeSettingsRepoForGetSettings implements settingsRepo, embedding it (left
// nil) so only the four methods GetSettings actually calls need overriding.
type fakeSettingsRepoForGetSettings struct {
	settingsRepo

	getUserSettingsFn func(
		ctx context.Context, userID string,
	) (*models.UserSettings, error)
	getLabelPresetsFn func(
		ctx context.Context, userID string, workspaceID *uuid.UUID,
	) (*models.LabelPresets, error)
	getURLPatternsFn func(
		ctx context.Context, userID string, workspaceID *uuid.UUID,
	) ([]models.URLPattern, error)
	getArchiveSettingsFn func(
		ctx context.Context, userID string,
	) (*models.ArchiveSettings, error)
}

func (f *fakeSettingsRepoForGetSettings) GetUserSettings(
	ctx context.Context, userID string,
) (*models.UserSettings, error) {
	return f.getUserSettingsFn(ctx, userID)
}

func (f *fakeSettingsRepoForGetSettings) GetLabelPresets(
	ctx context.Context, userID string, workspaceID *uuid.UUID,
) (*models.LabelPresets, error) {
	return f.getLabelPresetsFn(ctx, userID, workspaceID)
}

func (f *fakeSettingsRepoForGetSettings) GetURLPatterns(
	ctx context.Context, userID string, workspaceID *uuid.UUID,
) ([]models.URLPattern, error) {
	return f.getURLPatternsFn(ctx, userID, workspaceID)
}

func (f *fakeSettingsRepoForGetSettings) GetArchiveSettings(
	ctx context.Context, userID string,
) (*models.ArchiveSettings, error) {
	return f.getArchiveSettingsFn(ctx, userID)
}

type fakeSectionsLister struct {
	fn func(
		ctx context.Context, userID string, workspaceID *uuid.UUID,
	) ([]models.Section, error)
}

func (f *fakeSectionsLister) List(
	ctx context.Context, userID string, workspaceID *uuid.UUID,
) ([]models.Section, error) {
	return f.fn(ctx, userID, workspaceID)
}

type fakePoliciesLister struct {
	fn func(
		ctx context.Context, userID string, workspaceID *uuid.UUID,
	) ([]models.Policy, error)
}

func (f *fakePoliciesLister) List(
	ctx context.Context, userID string, workspaceID *uuid.UUID,
) ([]models.Policy, error) {
	return f.fn(ctx, userID, workspaceID)
}

type fakeWorkspacesLister struct {
	fn func(ctx context.Context, userID string) ([]models.Workspace, error)
}

func (f *fakeWorkspacesLister) List(
	ctx context.Context, userID string,
) ([]models.Workspace, error) {
	return f.fn(ctx, userID)
}

// noErrUserSettings/noErrWorkspaces/... build a fully-wired SettingsService
// whose dependencies all succeed with empty results, so a single field can
// be overridden per test to exercise one error-propagation branch at a time.
func newGetSettingsFixture() *SettingsService {
	return &SettingsService{
		settings: &fakeSettingsRepoForGetSettings{
			settingsRepo: nil,
			getUserSettingsFn: func(
				context.Context, string,
			) (*models.UserSettings, error) {
				//nolint:exhaustruct // unset fields are the fixture defaults
				return &models.UserSettings{UserID: "user-1"}, nil
			},
			getLabelPresetsFn: func(
				context.Context, string, *uuid.UUID,
			) (*models.LabelPresets, error) {
				//nolint:exhaustruct // unset fields are the fixture defaults
				return &models.LabelPresets{}, nil
			},
			getURLPatternsFn: func(
				context.Context, string, *uuid.UUID,
			) ([]models.URLPattern, error) {
				return nil, nil
			},
			getArchiveSettingsFn: func(
				context.Context, string,
			) (*models.ArchiveSettings, error) {
				//nolint:exhaustruct // unset fields are the fixture defaults
				return &models.ArchiveSettings{}, nil
			},
		},
		sections: &fakeSectionsLister{
			fn: func(
				context.Context, string, *uuid.UUID,
			) ([]models.Section, error) {
				return nil, nil
			},
		},
		policies: &fakePoliciesLister{
			fn: func(
				context.Context, string, *uuid.UUID,
			) ([]models.Policy, error) {
				return nil, nil
			},
		},
		workspaces: &fakeWorkspacesLister{
			fn: func(context.Context, string) ([]models.Workspace, error) {
				return nil, nil
			},
		},
	}
}

func TestGetSettings_Success(t *testing.T) {
	svc := newGetSettingsFixture()

	agg, err := svc.GetSettings(t.Context(), "user-1")
	require.NoError(t, err)
	require.NotNil(t, agg)
	assert.Equal(t, "user-1", agg.UserSettings.UserID)
}

func TestGetSettings_PropagatesUserSettingsError(t *testing.T) {
	wantErr := errors.New("db error")
	svc := newGetSettingsFixture()
	repo, ok := svc.settings.(*fakeSettingsRepoForGetSettings)
	require.True(t, ok)
	repo.getUserSettingsFn = func(context.Context, string) (*models.UserSettings, error) {
		return nil, wantErr
	}

	_, err := svc.GetSettings(t.Context(), "user-1")
	assert.ErrorIs(t, err, wantErr)
}

func TestGetSettings_PropagatesWorkspacesError(t *testing.T) {
	wantErr := errors.New("db error")
	svc := newGetSettingsFixture()
	svc.workspaces = &fakeWorkspacesLister{
		fn: func(context.Context, string) ([]models.Workspace, error) {
			return nil, wantErr
		},
	}

	_, err := svc.GetSettings(t.Context(), "user-1")
	assert.ErrorIs(t, err, wantErr)
}

func TestGetSettings_PropagatesLabelPresetsError(t *testing.T) {
	wantErr := errors.New("db error")
	svc := newGetSettingsFixture()
	repo, ok := svc.settings.(*fakeSettingsRepoForGetSettings)
	require.True(t, ok)
	repo.getLabelPresetsFn = func(
		context.Context, string, *uuid.UUID,
	) (*models.LabelPresets, error) {
		return nil, wantErr
	}

	_, err := svc.GetSettings(t.Context(), "user-1")
	assert.ErrorIs(t, err, wantErr)
}

func TestGetSettings_PropagatesURLPatternsError(t *testing.T) {
	wantErr := errors.New("db error")
	svc := newGetSettingsFixture()
	repo, ok := svc.settings.(*fakeSettingsRepoForGetSettings)
	require.True(t, ok)
	repo.getURLPatternsFn = func(
		context.Context, string, *uuid.UUID,
	) ([]models.URLPattern, error) {
		return nil, wantErr
	}

	_, err := svc.GetSettings(t.Context(), "user-1")
	assert.ErrorIs(t, err, wantErr)
}

func TestGetSettings_PropagatesArchiveSettingsError(t *testing.T) {
	wantErr := errors.New("db error")
	svc := newGetSettingsFixture()
	repo, ok := svc.settings.(*fakeSettingsRepoForGetSettings)
	require.True(t, ok)
	repo.getArchiveSettingsFn = func(
		context.Context, string,
	) (*models.ArchiveSettings, error) {
		return nil, wantErr
	}

	_, err := svc.GetSettings(t.Context(), "user-1")
	assert.ErrorIs(t, err, wantErr)
}

func TestGetSettings_PropagatesSectionsError(t *testing.T) {
	wantErr := errors.New("db error")
	svc := newGetSettingsFixture()
	svc.sections = &fakeSectionsLister{
		fn: func(context.Context, string, *uuid.UUID) ([]models.Section, error) {
			return nil, wantErr
		},
	}

	_, err := svc.GetSettings(t.Context(), "user-1")
	assert.ErrorIs(t, err, wantErr)
}

func TestGetSettings_PropagatesPoliciesError(t *testing.T) {
	wantErr := errors.New("db error")
	svc := newGetSettingsFixture()
	svc.policies = &fakePoliciesLister{
		fn: func(context.Context, string, *uuid.UUID) ([]models.Policy, error) {
			return nil, wantErr
		},
	}

	_, err := svc.GetSettings(t.Context(), "user-1")
	assert.ErrorIs(t, err, wantErr)
}
