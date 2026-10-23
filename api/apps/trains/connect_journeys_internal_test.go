package trains

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/irail"
)

// TestMapError pins the Connect code each journey-handler error maps to.
func TestMapError(t *testing.T) {
	assert.Equal(
		t, connect.CodeNotFound,
		connect.CodeOf(mapError(fmt.Errorf("%w: x", services.ErrUnknownStation))),
	)
	assert.Equal(
		t, connect.CodeUnavailable, connect.CodeOf(mapError(irail.ErrRateLimited)),
	)
	assert.Equal(
		t, connect.CodeInternal, connect.CodeOf(mapError(errors.New("boom"))),
	)
}
