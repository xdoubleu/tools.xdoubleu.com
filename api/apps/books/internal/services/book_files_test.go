//nolint:testpackage // testing unexported service helpers
package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- titleFromFilename ---

func TestTitleFromFilename(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"Black Hat Go.pdf", "Black Hat Go"},
		{"Attacking Network Protocols.pdf", "Attacking Network Protocols"},
		{
			"andrew-ng-machine-learning-yearning.pdf",
			"andrew ng machine learning yearning",
		},
		{"ebook-UndisturbedREST_v1.pdf", "ebook UndisturbedREST v1"},
		{"no-meta.pdf", "no meta"},
		{".pdf", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, titleFromFilename(c.filename), c.filename)
	}
}
