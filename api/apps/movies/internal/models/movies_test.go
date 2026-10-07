package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/movies/internal/models"
)

func TestIsRating(t *testing.T) {
	assert.True(t, models.IsRating(nil), "unrated")
	for _, n := range []int{1, 3, 5} {
		assert.True(t, models.IsRating(&n), n)
	}
	for _, n := range []int{0, 6, -1} {
		assert.False(t, models.IsRating(&n), n)
	}
}
