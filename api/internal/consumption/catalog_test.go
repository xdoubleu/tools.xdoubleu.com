package consumption_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/consumption"
)

func TestSameTitle(t *testing.T) {
	tests := []struct {
		a, b string
		same bool
	}{
		{"Dune", "dune", true},
		{"The Left Hand of Darkness", "Left Hand of Darkness", true},
		{"Outer Wilds", "Outer  Wilds!", true},
		{"Hades II", "Hades II", true},
		{"Children of Time: A Novel", "Children of Time", true},
		{"Children of Time", "Children of Time (2015)", true},
		{"Severance — Season 1", "Severance", true},
		{"Ender's Game", "Enders Game", true},
		{"Dune", "Dune Messiah", false},
		{"A", "The", false},
		{"", "", false},
		{"Arrival", "", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.same, consumption.SameTitle(tt.a, tt.b), "%q vs %q", tt.a, tt.b)
		assert.Equal(t, tt.same, consumption.SameTitle(tt.b, tt.a), "%q vs %q", tt.b, tt.a)
	}
}
