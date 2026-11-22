//nolint:exhaustruct // sparse fixtures
package models_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/learningpaths/internal/models"
)

func TestProgressOf(t *testing.T) {
	m1, m2 := uuid.New(), uuid.New()
	lp := &models.LearningPath{
		ID:    uuid.New(),
		Title: "Go",
		Modules: []models.Module{
			{ID: m1, Title: "One", Items: []models.Item{{Completed: true}, {}}},
			{ID: m2, Title: "Two"},
		},
	}

	p := models.ProgressOf(lp)
	assert.Equal(t, lp.ID, p.PathID)
	assert.Equal(t, int32(2), p.TotalItems)
	assert.Equal(t, int32(1), p.CompletedItems)
	require.Len(t, p.Modules, 2)
	assert.Equal(t, m1, p.Modules[0].ID)
	assert.Equal(t, int32(2), p.Modules[0].TotalItems)
	assert.Equal(t, int32(1), p.Modules[0].CompletedItems)
	assert.Equal(t, int32(0), p.Modules[1].TotalItems)
}
