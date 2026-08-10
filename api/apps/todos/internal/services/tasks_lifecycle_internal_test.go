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

// fakeTasksRepo implements tasksRepo, embedding the interface (left nil) so
// only the methods a test configures need overriding; calling an
// unconfigured method panics loudly rather than silently hitting a real DB.
type fakeTasksRepo struct {
	tasksRepo

	listOpenFn func(
		ctx context.Context,
		userID string,
		sectionID, workspaceID *uuid.UUID,
		limit, offset int32,
	) ([]models.Task, bool, error)
	listByStatusFn func(
		ctx context.Context,
		userID string,
		status string,
		workspaceID *uuid.UUID,
	) ([]models.Task, error)
	listArchivedFn func(
		ctx context.Context,
		userID string,
		query string,
		workspaceID *uuid.UUID,
	) ([]models.Task, error)
	searchAllFn func(
		ctx context.Context,
		userID string,
		query string,
		workspaceID *uuid.UUID,
	) ([]models.Task, error)
}

func (f *fakeTasksRepo) ListOpen(
	ctx context.Context,
	userID string,
	sectionID, workspaceID *uuid.UUID,
	limit, offset int32,
) ([]models.Task, bool, error) {
	if f.listOpenFn != nil {
		return f.listOpenFn(ctx, userID, sectionID, workspaceID, limit, offset)
	}
	return nil, false, nil
}

func (f *fakeTasksRepo) ListByStatus(
	ctx context.Context,
	userID string,
	status string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	if f.listByStatusFn != nil {
		return f.listByStatusFn(ctx, userID, status, workspaceID)
	}
	return nil, nil
}

func (f *fakeTasksRepo) ListArchived(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	if f.listArchivedFn != nil {
		return f.listArchivedFn(ctx, userID, query, workspaceID)
	}
	return nil, nil
}

func (f *fakeTasksRepo) SearchAll(
	ctx context.Context,
	userID string,
	query string,
	workspaceID *uuid.UUID,
) ([]models.Task, error) {
	if f.searchAllFn != nil {
		return f.searchAllFn(ctx, userID, query, workspaceID)
	}
	return nil, nil
}

// fakeSettingsForTasks implements settingsRepo, embedding it (left nil) so
// TaskService's URL-pattern shortcut lookups (searchByShortcut,
// enrichWithShortcuts) safely see "no patterns configured" by default.
type fakeSettingsForTasks struct {
	settingsRepo
}

func (f *fakeSettingsForTasks) GetURLPatterns(
	_ context.Context, _ string, _ *uuid.UUID,
) ([]models.URLPattern, error) {
	return nil, nil
}

func TestListByRequestedStatus_Done(t *testing.T) {
	//nolint:exhaustruct // only the configured Fn fields matter
	tasks := &fakeTasksRepo{
		listByStatusFn: func(
			_ context.Context, _ string, status string, _ *uuid.UUID,
		) ([]models.Task, error) {
			assert.Equal(t, models.StatusDone, status)
			return nil, nil
		},
	}
	svc := &TaskService{
		tasks:    tasks,
		settings: &fakeSettingsForTasks{}, //nolint:exhaustruct // no fields to set
		sections: nil,
	}

	_, hasMore, err := svc.ListByRequestedStatus(
		t.Context(), "user-1", "done", nil, nil, 0, 0,
	)
	require.NoError(t, err)
	assert.False(t, hasMore)
}

func TestListByRequestedStatus_Archived(t *testing.T) {
	//nolint:exhaustruct // only the configured Fn fields matter
	tasks := &fakeTasksRepo{
		listArchivedFn: func(
			_ context.Context, _ string, query string, _ *uuid.UUID,
		) ([]models.Task, error) {
			assert.Empty(t, query)
			return nil, nil
		},
	}
	svc := &TaskService{
		tasks:    tasks,
		settings: &fakeSettingsForTasks{}, //nolint:exhaustruct // no fields to set
		sections: nil,
	}

	_, hasMore, err := svc.ListByRequestedStatus(
		t.Context(), "user-1", "archived", nil, nil, 0, 0,
	)
	require.NoError(t, err)
	assert.False(t, hasMore)
}

func TestListByRequestedStatus_DefaultUsesListOpen(t *testing.T) {
	wantHasMore := true
	//nolint:exhaustruct // only the configured Fn fields matter
	tasks := &fakeTasksRepo{
		listOpenFn: func(
			context.Context, string, *uuid.UUID, *uuid.UUID, int32, int32,
		) ([]models.Task, bool, error) {
			return nil, wantHasMore, nil
		},
	}
	svc := &TaskService{
		tasks:    tasks,
		settings: &fakeSettingsForTasks{}, //nolint:exhaustruct // no fields to set
		sections: nil,
	}

	_, hasMore, err := svc.ListByRequestedStatus(
		t.Context(), "user-1", "", nil, nil, 10, 0,
	)
	require.NoError(t, err)
	assert.Equal(t, wantHasMore, hasMore)
}

func TestListByRequestedStatus_PropagatesError(t *testing.T) {
	wantErr := errors.New("db error")
	//nolint:exhaustruct // only the configured Fn fields matter
	tasks := &fakeTasksRepo{
		listOpenFn: func(
			context.Context, string, *uuid.UUID, *uuid.UUID, int32, int32,
		) ([]models.Task, bool, error) {
			return nil, false, wantErr
		},
	}
	svc := &TaskService{
		tasks:    tasks,
		settings: &fakeSettingsForTasks{}, //nolint:exhaustruct // no fields to set
		sections: nil,
	}

	_, _, err := svc.ListByRequestedStatus(t.Context(), "user-1", "", nil, nil, 10, 0)
	assert.ErrorIs(t, err, wantErr)
}
