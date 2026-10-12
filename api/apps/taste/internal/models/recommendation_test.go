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

func TestCanAccept(t *testing.T) {
	for _, tt := range []struct {
		domain, ref string
		want        bool
	}{
		{"books", "hardcover:978", true},
		{"movies", "movie:1", true},
		{"series", "series:2", true},
		{"books", "", false},
		{"games", "620", false},
		{"podcasts", "1", false},
		{"feeds", "https://x.example", false},
	} {
		//nolint:exhaustruct // what CanAccept reads
		r := models.Recommendation{Domain: tt.domain, CatalogRef: tt.ref}
		assert.Equal(t, tt.want, r.CanAccept(), tt.domain+" "+tt.ref)
	}
}

func TestIsFeedback(t *testing.T) {
	for _, f := range []string{"", "hit", "miss", "not_interested", "known"} {
		assert.True(t, models.IsFeedback(f), f)
	}
	assert.False(t, models.IsFeedback("meh"))
}
