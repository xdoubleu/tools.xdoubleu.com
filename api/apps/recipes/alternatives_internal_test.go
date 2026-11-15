package recipes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDtoToRecipe_AlternativeGroups(t *testing.T) {
	_, ings := dtoToRecipe(
		"Bowl", nil, 2,
		[]string{"rijst", "pasta", "ui"},
		[]float64{200, 250, 1},
		[]string{"g", "g", ""},
		nil,
		[]string{" base ", "base"},
	)

	require.Len(t, ings, 3)
	require.NotNil(t, ings[0].AlternativeGroup)
	assert.Equal(t, "base", *ings[0].AlternativeGroup)
	require.NotNil(t, ings[1].AlternativeGroup)
	assert.Equal(t, "base", *ings[1].AlternativeGroup)
	assert.Nil(t, ings[2].AlternativeGroup)
}
