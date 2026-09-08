package trains

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/trains/internal/services"
	"tools.xdoubleu.com/apps/trains/pkg/csa"
)

// TestMapError pins the Connect code each journey-handler error maps to —
// notably ErrRouterWarmingUp -> CodeUnavailable, added with issue #1484.
func TestMapError(t *testing.T) {
	assert.Equal(
		t, connect.CodeNotFound, connect.CodeOf(mapError(csa.ErrUnknownStop)),
	)
	assert.Equal(
		t,
		connect.CodeUnavailable,
		connect.CodeOf(mapError(services.ErrRouterWarmingUp)),
	)
	assert.Equal(
		t, connect.CodeInternal, connect.CodeOf(mapError(errors.New("boom"))),
	)
}
