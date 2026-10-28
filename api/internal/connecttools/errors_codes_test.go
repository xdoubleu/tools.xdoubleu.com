package connecttools_test

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/connecttools"
	"tools.xdoubleu.com/internal/database"
)

var errAppSentinel = errors.New("app sentinel")

func TestMapError_AppCode(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		fmt.Errorf("wrapped: %w", errAppSentinel),
		connecttools.Code(errAppSentinel, connect.CodeFailedPrecondition),
	)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.ErrorIs(t, err, errAppSentinel)

	var connectErr *connect.Error
	assert.ErrorAs(t, err, &connectErr)
	assert.Equal(t, "wrapped: app sentinel", connectErr.Message())
}

func TestMapError_AppCodeWithMessage(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		database.ErrResourceConflict,
		connecttools.CodeWithMessage(
			database.ErrResourceConflict, connect.CodeAlreadyExists, "feed exists",
		),
	)
	var connectErr *connect.Error
	assert.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
	assert.Equal(t, "feed exists", connectErr.Message())
}

func TestMapError_AppCodeNoMatchFallsBack(t *testing.T) {
	t.Parallel()

	err := connecttools.MapError(
		database.ErrResourceNotFound,
		connecttools.Code(errAppSentinel, connect.CodeUnavailable),
	)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestMapError_ConnectErrorPassesThrough(t *testing.T) {
	t.Parallel()

	in := connect.NewError(connect.CodeFailedPrecondition, "nope")
	err := connecttools.MapError(fmt.Errorf("ctx: %w", in))
	assert.Same(t, in, err)
}
