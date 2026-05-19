package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/todos/internal/models"
)

func TestLabelPresets_Values_Empty(t *testing.T) {
	lp := &models.LabelPresets{Labels: []models.LabelPreset{}}
	values := lp.Values()
	assert.NotNil(t, values)
	assert.Len(t, values, 0)
}

func TestLabelPresets_Values_Populated(t *testing.T) {
	lp := &models.LabelPresets{
		Labels: []models.LabelPreset{
			{Value: "bug", Color: "#ff0000"},
			{Value: "feature", Color: "#00ff00"},
		},
	}
	values := lp.Values()
	assert.Equal(t, []string{"bug", "feature"}, values)
}
