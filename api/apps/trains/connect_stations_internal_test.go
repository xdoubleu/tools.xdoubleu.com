package trains

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCount32 checks the clamp can't wrap negative.
func TestCount32(t *testing.T) {
	assert.Equal(t, int32(0), count32(0))
	assert.Equal(t, int32(2887), count32(2887))
	assert.Equal(t, int32(math.MaxInt32), count32(math.MaxInt32))
	assert.Equal(t, int32(math.MaxInt32), count32(math.MaxInt32+1))
	assert.Equal(t, int32(0), count32(-1))
}
