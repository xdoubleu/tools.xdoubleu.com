package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/taste/internal/models"
)

func TestIsTrack(t *testing.T) {
	assert.True(t, models.IsTrack("more"))
	assert.True(t, models.IsTrack("broaden"))
	assert.False(t, models.IsTrack("sideways"))
	assert.False(t, models.IsTrack(""))
}
