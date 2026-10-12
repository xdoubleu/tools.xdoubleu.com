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
		{"Ender's Game", "Enders Game", true},
		{"Ender’s Game", "Enders Game", true},
		{"“Dune”", "Dune", true},
		{"Dune", "Dune: Part Two", false},
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

func TestBestTitle(t *testing.T) {
	titles := []string{"Halo: Reach", "Halo", "Children of Time: A Novel"}
	assert.Equal(t, 1, consumption.BestTitle(titles, "halo"), "exact beats subtitle")
	assert.Equal(t, 0, consumption.BestTitle(titles, "Halo Reach"))
	assert.Equal(t, 2, consumption.BestTitle(titles, "Children of Time"))
	assert.Equal(t, 1, consumption.BestTitle(
		[]string{"Severance", "Children of Time"}, "Children of Time (2015)",
	))
	assert.Equal(t, 1, consumption.BestTitle(
		[]string{"Severance", "Children of Time"}, "Children of Time — Book One",
	))
	assert.Equal(t, -1, consumption.BestTitle(titles, "Dune"))
	assert.Equal(t, 0, consumption.BestTitle([]string{"Arrival"}, "Arrival: (2016)"))
	assert.Equal(t, -1, consumption.BestTitle([]string{"Arrival"}, ":"))
	assert.Equal(t, -1, consumption.BestTitle(titles, ""))
	assert.Equal(t, -1, consumption.BestTitle(nil, "Halo"))
}
