package tmdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackoffDelayDoubles(t *testing.T) {
	assert.Equal(t, backoffBase, backoffDelay(0))
	assert.Equal(t, 4*backoffBase, backoffDelay(2))
}

func TestPositive(t *testing.T) {
	assert.Nil(t, positive(0))
	assert.Nil(t, positive(-1))
	got := positive(1)
	require.NotNil(t, got)
	assert.Equal(t, 1, *got)
}
