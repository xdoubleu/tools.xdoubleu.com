package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/apps/backlog/internal/models"
)

func TestIsSpecialTag(t *testing.T) {
	assert.True(t, models.IsSpecialTag(models.TagOwnPhysical))
	assert.True(t, models.IsSpecialTag(models.TagOwnDigital))
	assert.True(t, models.IsSpecialTag(models.TagFavourite))
	assert.False(t, models.IsSpecialTag("classics"))
	assert.False(t, models.IsSpecialTag(""))
}

func TestUserBook_HasTag(t *testing.T) {
	ub := models.UserBook{ //nolint:exhaustruct //only Tags needed
		Tags: []string{"classics", models.TagFavourite},
	}
	assert.True(t, ub.HasTag("classics"))
	assert.True(t, ub.HasTag(models.TagFavourite))
	assert.False(t, ub.HasTag("sci-fi"))
	assert.False(t, ub.HasTag(""))
}

func TestUserBook_HasTag_Empty(t *testing.T) {
	ub := models.UserBook{} //nolint:exhaustruct //no tags
	assert.False(t, ub.HasTag("any"))
}

func TestUserBook_DisplayTags(t *testing.T) {
	ub := models.UserBook{ //nolint:exhaustruct //only Tags needed
		Tags: []string{
			models.TagOwnPhysical,
			"classics",
			models.TagFavourite,
			models.TagOwnDigital,
			"sci-fi",
		},
	}
	got := ub.DisplayTags()
	assert.Equal(t, []string{"classics", "sci-fi"}, got)
}

func TestUserBook_DisplayTags_AllSpecial(t *testing.T) {
	ub := models.UserBook{ //nolint:exhaustruct //only Tags needed
		Tags: []string{
			models.TagOwnPhysical,
			models.TagOwnDigital,
			models.TagFavourite,
		},
	}
	assert.Empty(t, ub.DisplayTags())
}

func TestUserBook_DisplayTags_Empty(t *testing.T) {
	ub := models.UserBook{} //nolint:exhaustruct //no tags
	assert.Empty(t, ub.DisplayTags())
}
