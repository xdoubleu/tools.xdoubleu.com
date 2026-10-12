package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/logging"
)

func callScrubbed(t *testing.T, nextErr error) error {
	t.Helper()

	next := func(
		_ context.Context,
		_ connect.Spec,
		_ connect.ServerStream,
	) error {
		return nextErr
	}

	return ScrubInternalErrors(logging.NewNopLogger())(next)(
		context.Background(),
		connect.Spec{
			StreamType:       connect.StreamTypeUnary,
			Schema:           nil,
			Procedure:        "/test.v1.Service/Method",
			IdempotencyLevel: connect.IdempotencyUnknown,
		},
		nil,
	)
}

func TestScrubInternalErrorsInternal(t *testing.T) {
	err := callScrubbed(
		t,
		connect.NewError(connect.CodeInternal, "secret detail"),
	)

	require.Error(t, err)
	assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	assert.NotContains(t, err.Error(), "secret detail")
	assert.Contains(t, err.Error(), "internal server error")
}

func TestScrubInternalErrorsUnknown(t *testing.T) {
	err := callScrubbed(t, errors.New("raw secret detail"))

	require.Error(t, err)
	assert.Equal(t, connect.CodeUnknown, connect.CodeOf(err))
	assert.NotContains(t, err.Error(), "raw secret detail")
}

func TestScrubInternalErrorsPassesThroughOtherCodes(t *testing.T) {
	original := connect.NewError(
		connect.CodeNotFound,
		"recipe not found",
	)
	err := callScrubbed(t, original)

	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	assert.Contains(t, err.Error(), "recipe not found")
}

func TestScrubInternalErrorsPassesThroughSuccess(t *testing.T) {
	err := callScrubbed(t, nil)
	assert.NoError(t, err)
}

func TestScrubInternalErrorsContextCanceled(t *testing.T) {
	cause := fmt.Errorf("query failed: %w", context.Canceled)
	wrapped := connect.NewError(connect.CodeInternal, cause.Error()).WithCause(cause)
	err := callScrubbed(t, wrapped)

	require.Error(t, err)
	assert.Equal(t, connect.CodeCanceled, connect.CodeOf(err))
	assert.ErrorIs(t, err, context.Canceled)
}
