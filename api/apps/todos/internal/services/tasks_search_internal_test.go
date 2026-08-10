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

func TestBucketTasksByStatus(t *testing.T) {
	tasks := []models.Task{
		{Status: models.StatusOpen},     //nolint:exhaustruct // test fields only
		{Status: models.StatusDone},     //nolint:exhaustruct // test fields only
		{Status: models.StatusArchived}, //nolint:exhaustruct // test fields only
		{Status: models.StatusDone},     //nolint:exhaustruct // test fields only
	}

	open, done, archived := bucketTasksByStatus(tasks)
	assert.Len(t, open, 1)
	assert.Len(t, done, 2)
	assert.Len(t, archived, 1)
}

func TestBucketTasksByStatus_Empty(t *testing.T) {
	open, done, archived := bucketTasksByStatus(nil)
	assert.Empty(t, open)
	assert.Empty(t, done)
	assert.Empty(t, archived)
}

func TestSearchBucketed_SplitsByStatus(t *testing.T) {
	//nolint:exhaustruct // only the configured Fn fields matter
	tasks := &fakeTasksRepo{
		//nolint:exhaustruct // only searchAllFn matters for this test
		searchAllFn: func(
			context.Context, string, string, *uuid.UUID,
		) ([]models.Task, error) {
			return []models.Task{
				{
					Status: models.StatusOpen,
				}, //nolint:exhaustruct // test fields only
				{
					Status: models.StatusDone,
				}, //nolint:exhaustruct // test fields only
				{
					Status: models.StatusArchived,
				}, //nolint:exhaustruct // test fields only
			}, nil
		},
	}
	svc := &TaskService{
		tasks:    tasks,
		settings: &fakeSettingsForTasks{}, //nolint:exhaustruct // no fields to set
		sections: nil,
	}

	open, done, archived, err := svc.SearchBucketed(t.Context(), "user-1", "q", nil)
	require.NoError(t, err)
	assert.Len(t, open, 1)
	assert.Len(t, done, 1)
	assert.Len(t, archived, 1)
}

func TestSearchBucketed_PropagatesError(t *testing.T) {
	wantErr := errors.New("db error")
	//nolint:exhaustruct // only the configured Fn fields matter
	tasks := &fakeTasksRepo{

		searchAllFn: func(
			context.Context, string, string, *uuid.UUID,
		) ([]models.Task, error) {
			return nil, wantErr
		},
	}
	svc := &TaskService{
		tasks:    tasks,
		settings: &fakeSettingsForTasks{}, //nolint:exhaustruct // no fields to set
		sections: nil,
	}

	_, _, _, err := svc.SearchBucketed(t.Context(), "user-1", "q", nil)
	assert.ErrorIs(t, err, wantErr)
}
