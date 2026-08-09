package logging_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/logging"
)

func TestErrAttr(t *testing.T) {
	t.Parallel()

	attr := logging.ErrAttr(errors.New("boom"))
	assert.Equal(t, "error", attr.Key)

	err, ok := attr.Value.Any().(error)
	require.True(t, ok)
	assert.Equal(t, "boom", err.Error())
}

func TestNewNopLogger(t *testing.T) {
	t.Parallel()

	logger := logging.NewNopLogger()
	assert.NotNil(t, logger)
	logger.Info("this should go nowhere")
}
