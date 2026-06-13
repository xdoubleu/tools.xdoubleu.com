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

func intPtr(i int) *int { return &i }

func TestUserBook_DisplayProgressPercent(t *testing.T) {
	pages := func(p *int) *models.Book {
		return &models.Book{PageCount: p} //nolint:exhaustruct //only PageCount
	}
	tests := []struct {
		name string
		ub   models.UserBook
		want int
	}{
		{
			name: "percent mode returns stored percent",
			ub: models.UserBook{ //nolint:exhaustruct //fields under test
				ProgressMode:    models.ProgressModePercent,
				ProgressPercent: 60,
			},
			want: 60,
		},
		{
			name: "percent mode clamps above 100",
			ub: models.UserBook{ //nolint:exhaustruct //fields under test
				ProgressMode:    models.ProgressModePercent,
				ProgressPercent: 140,
			},
			want: 100,
		},
		{
			name: "pages mode derives percent",
			ub: models.UserBook{ //nolint:exhaustruct //fields under test
				ProgressMode: models.ProgressModePages,
				CurrentPage:  150,
				Book:         pages(intPtr(300)),
			},
			want: 50,
		},
		{
			name: "pages mode rounds",
			ub: models.UserBook{ //nolint:exhaustruct //fields under test
				ProgressMode: models.ProgressModePages,
				CurrentPage:  1,
				Book:         pages(intPtr(3)),
			},
			want: 33,
		},
		{
			name: "pages mode clamps above 100",
			ub: models.UserBook{ //nolint:exhaustruct //fields under test
				ProgressMode: models.ProgressModePages,
				CurrentPage:  400,
				Book:         pages(intPtr(300)),
			},
			want: 100,
		},
		{
			name: "pages mode without page count returns zero",
			ub: models.UserBook{ //nolint:exhaustruct //fields under test
				ProgressMode: models.ProgressModePages,
				CurrentPage:  150,
				Book:         pages(nil),
			},
			want: 0,
		},
		{
			name: "pages mode without book returns zero",
			ub: models.UserBook{ //nolint:exhaustruct //fields under test
				ProgressMode: models.ProgressModePages,
				CurrentPage:  150,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ub.DisplayProgressPercent())
		})
	}
}
