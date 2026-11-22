package protomap_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/apps/books/internal/protomap"
)

func TestSeriesFromProto(t *testing.T) {
	assert.Nil(t, protomap.SeriesFromProto("  ", nil))

	pos := func(f float64) *float64 { return &f }
	for _, tc := range []struct {
		in   *float64
		want *float64
	}{
		{nil, nil},
		{pos(0), pos(0)},
		{pos(1.5), pos(1.5)},
		{pos(-1), nil},
		{pos(math.NaN()), nil},
		{pos(math.Inf(1)), nil},
	} {
		got := protomap.SeriesFromProto(" Discworld ", tc.in)
		require.NotNil(t, got)
		assert.Equal(t, "Discworld", got.Name)
		assert.Equal(t, tc.want, got.Position)
		assert.Nil(t, got.Total)
	}
}
