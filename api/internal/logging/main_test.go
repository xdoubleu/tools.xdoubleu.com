package logging_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/internal/logging"
)

func TestNewBufLogHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	handler := logging.NewBufLogHandler(&buf, nil)
	assert.NotNil(t, handler)
	assert.True(t, handler.Enabled(t.Context(), 0))
}
