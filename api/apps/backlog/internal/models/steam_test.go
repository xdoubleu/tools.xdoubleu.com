package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/backlog/internal/models"
)

func TestAchievement_HasGlobalPercent_Nil(t *testing.T) {
	a := models.Achievement{} //nolint:exhaustruct //only GlobalPercent matters
	assert.False(t, a.HasGlobalPercent())
}

func TestAchievement_HasGlobalPercent_NonNil(t *testing.T) {
	pct := 42.0
	a := models.Achievement{ //nolint:exhaustruct //only GlobalPercent matters
		GlobalPercent: &pct,
	}
	assert.True(t, a.HasGlobalPercent())
}

func TestAchievement_GlobalPercentValue_Nil(t *testing.T) {
	a := models.Achievement{} //nolint:exhaustruct //only GlobalPercent matters
	assert.Equal(t, 0.0, a.GlobalPercentValue())
}

func TestAchievement_GlobalPercentValue_NonNil(t *testing.T) {
	pct := 55.5
	a := models.Achievement{ //nolint:exhaustruct //only GlobalPercent matters
		GlobalPercent: &pct,
	}
	assert.Equal(t, 55.5, a.GlobalPercentValue())
}

func TestGame_SetCalculatedInfo_ZeroAchievements(t *testing.T) {
	g := models.Game{} //nolint:exhaustruct //fields set by SetCalculatedInfo
	g.SetCalculatedInfo([]models.Achievement{}, 5)
	assert.Equal(t, "0.00", g.CompletionRate)
	assert.Equal(t, "0.0000", g.Contribution)
}
