package protomap_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/recipes/internal/protomap"
)

func TestToFraction_Zero(t *testing.T) {
	assert.Equal(t, "0", protomap.ToFraction(0.0))
}

func TestToFraction_Eighth(t *testing.T) {
	assert.Equal(t, "⅛", protomap.ToFraction(0.125))
}

func TestToFraction_Quarter(t *testing.T) {
	assert.Equal(t, "¼", protomap.ToFraction(0.25))
}

func TestToFraction_Half(t *testing.T) {
	assert.Equal(t, "½", protomap.ToFraction(0.5))
}

func TestToFraction_ThreeQuarters(t *testing.T) {
	assert.Equal(t, "¾", protomap.ToFraction(0.75))
}

func TestToFraction_OneAndHalf(t *testing.T) {
	assert.Equal(t, "1½", protomap.ToFraction(1.5))
}

func TestToFraction_WholeNumber(t *testing.T) {
	assert.Equal(t, "3", protomap.ToFraction(3.0))
}

func TestToFraction_TwoAndThreeEighths(t *testing.T) {
	assert.Equal(t, "2⅜", protomap.ToFraction(2.375))
}

func TestToFraction_RoundsUp(t *testing.T) {
	// 0.9375 rounds to 1 whole
	assert.Equal(t, "1", protomap.ToFraction(0.9375))
}

func TestToFraction_OneThird(t *testing.T) {
	assert.Equal(t, "⅓", protomap.ToFraction(1.0/3))
}

func TestToFraction_TwoThirds(t *testing.T) {
	assert.Equal(t, "⅔", protomap.ToFraction(2.0/3))
}

func TestToFraction_OneAndOneThird(t *testing.T) {
	assert.Equal(t, "1⅓", protomap.ToFraction(1.0+1.0/3))
}

func TestToFractionCeiling_Zero(t *testing.T) {
	assert.Equal(t, "0", protomap.ToFractionCeiling(0.0))
}

func TestToFractionCeiling_ExactFraction(t *testing.T) {
	assert.Equal(t, "½", protomap.ToFractionCeiling(0.5))
}

func TestToFractionCeiling_ExactOneThird(t *testing.T) {
	assert.Equal(t, "⅓", protomap.ToFractionCeiling(1.0/3))
}

func TestToFractionCeiling_ExactTwoThirds(t *testing.T) {
	assert.Equal(t, "⅔", protomap.ToFractionCeiling(2.0/3))
}

func TestToFractionCeiling_BetweenFractions(t *testing.T) {
	// 0.4 is between ⅓ and ½ — should ceiling to ½
	assert.Equal(t, "½", protomap.ToFractionCeiling(0.4))
}

func TestToFractionCeiling_SmallAmount(t *testing.T) {
	// 0.1 is between 0 and ⅛ — should ceiling to ⅛
	assert.Equal(t, "⅛", protomap.ToFractionCeiling(0.1))
}

func TestToFractionCeiling_WholeNumber(t *testing.T) {
	assert.Equal(t, "3", protomap.ToFractionCeiling(3.0))
}

func TestToFractionCeiling_OneAndFractional(t *testing.T) {
	// 1.1 ceilings to 1⅛
	assert.Equal(t, "1⅛", protomap.ToFractionCeiling(1.1))
}

func TestToFractionCeiling_NearWholeRoundsUp(t *testing.T) {
	// 0.9 is between ⅞ and 1 — should ceiling to 1
	assert.Equal(t, "1", protomap.ToFractionCeiling(0.9))
}

func TestToFractionCeiling_TwoAndOneThird(t *testing.T) {
	assert.Equal(t, "2⅓", protomap.ToFractionCeiling(2.0+1.0/3))
}
