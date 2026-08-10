package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
