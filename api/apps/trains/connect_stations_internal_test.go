package trains

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// count32 narrows a parsed count to the proto's int32 field. The clamp is
// unreachable against any real GTFS feed, so it is asserted directly rather
// than through a fixture: what matters is that an implausible count cannot
// wrap into a negative one, which would read as a coverage regression
// instead of an overflow (issue #1459).
func TestCount32(t *testing.T) {
	assert.Equal(t, int32(0), count32(0))
	assert.Equal(t, int32(2887), count32(2887))
	assert.Equal(t, int32(math.MaxInt32), count32(math.MaxInt32))
	assert.Equal(t, int32(math.MaxInt32), count32(math.MaxInt32+1))
	assert.Equal(t, int32(0), count32(-1))
}
