package logging_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/gateway/internal/logging"
)

func TestPrefixWriter_PrefixesEachCompleteLine(t *testing.T) {
	t.Parallel()

	var dst bytes.Buffer
	w := logging.NewPrefixWriter("api", &dst)

	n, err := w.Write([]byte("hello\nworld\n"))

	assert.NoError(t, err)
	assert.Equal(t, 12, n)
	assert.Equal(t, "[api] hello\n[api] world\n", dst.String())
}

func TestPrefixWriter_BuffersPartialLineAcrossWrites(t *testing.T) {
	t.Parallel()

	var dst bytes.Buffer
	w := logging.NewPrefixWriter("web", &dst)

	_, err := w.Write([]byte("hel"))
	assert.NoError(t, err)
	assert.Empty(t, dst.String(), "partial line must not be flushed yet")

	_, err = w.Write([]byte("lo\n"))
	assert.NoError(t, err)
	assert.Equal(t, "[web] hello\n", dst.String())
}
