package recipes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"tools.xdoubleu.com/internal/templates"
)

func TestToFraction_Zero(t *testing.T) {
	assert.Equal(t, "0", templates.ToFraction(0.0))
}

func TestToFraction_Eighth(t *testing.T) {
	assert.Equal(t, "⅛", templates.ToFraction(0.125))
}

func TestToFraction_Quarter(t *testing.T) {
	assert.Equal(t, "¼", templates.ToFraction(0.25))
}

func TestToFraction_Half(t *testing.T) {
	assert.Equal(t, "½", templates.ToFraction(0.5))
}

func TestToFraction_ThreeQuarters(t *testing.T) {
	assert.Equal(t, "¾", templates.ToFraction(0.75))
}

func TestToFraction_OneAndHalf(t *testing.T) {
	assert.Equal(t, "1½", templates.ToFraction(1.5))
}

func TestToFraction_WholeNumber(t *testing.T) {
	assert.Equal(t, "3", templates.ToFraction(3.0))
}

func TestToFraction_TwoAndThreeEighths(t *testing.T) {
	assert.Equal(t, "2⅜", templates.ToFraction(2.375))
}

func TestToFraction_RoundsUp(t *testing.T) {
	// 0.9375 rounds to 1 whole
	assert.Equal(t, "1", templates.ToFraction(0.9375))
}
