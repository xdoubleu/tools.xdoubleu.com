package testtools_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/gateway/internal/testtools"
)

func TestErrorWriter_Write(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writer := testtools.ErrorWriter{ResponseWriter: rec}

	n, err := writer.Write([]byte("hello"))

	assert.Equal(t, 0, n)
	require.Error(t, err)
	assert.Equal(t, "write failed", err.Error())
}
