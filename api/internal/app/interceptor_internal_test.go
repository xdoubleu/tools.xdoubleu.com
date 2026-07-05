package app

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xdoubleu/essentia/v4/pkg/logging"
)

func callScrubbed(t *testing.T, nextErr error) error {
	t.Helper()

	next := func(
		_ context.Context,
		_ connect.AnyRequest,
	) (connect.AnyResponse, error) {
		return nil, nextErr
	}

	req := connect.NewRequest(&struct{}{})
	_, err := scrubInterceptor(logging.NewNopLogger())(next)(
		context.Background(),
		req,
	)
	return err
}

func TestScrubInternalErrorsInternal(t *testing.T) {
	err := callScrubbed(
		t,
		connect.NewError(connect.CodeInternal, errors.New("secret detail")),
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
		errors.New("recipe not found"),
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
