package trains

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/apps/trains/internal/services"
	trainsv1 "tools.xdoubleu.com/gen/trains/v1"
)

// TestSavedCommutes_Unauthenticated pins that every saved-commute RPC
// rejects a call with no user in context — the AppAccess middleware is the
// real gate, this is the handler's own belt-and-braces check.
func TestSavedCommutes_Unauthenticated(t *testing.T) {
	h := &trainsConnectHandler{app: nil}
	ctx := context.Background()

	_, err := h.ListSavedCommutes(ctx, connect.NewRequest(
		&trainsv1.ListSavedCommutesRequest{},
	))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	_, err = h.CreateSavedCommute(ctx, connect.NewRequest(
		&trainsv1.CreateSavedCommuteRequest{},
	))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	_, err = h.UpdateSavedCommute(ctx, connect.NewRequest(
		&trainsv1.UpdateSavedCommuteRequest{},
	))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	_, err = h.DeleteSavedCommute(ctx, connect.NewRequest(
		&trainsv1.DeleteSavedCommuteRequest{},
	))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// TestMapError_SavedCommute pins the saved-commute error mappings.
func TestMapError_SavedCommute(t *testing.T) {
	assert.Equal(
		t, connect.CodeInvalidArgument,
		connect.CodeOf(mapError(services.ErrInvalidCommute)),
	)
}
