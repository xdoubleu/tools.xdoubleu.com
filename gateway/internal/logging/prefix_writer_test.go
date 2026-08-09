package logging_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/gateway/internal/logging"
)

// erroringWriter fails starting from its failAfter'th Write call, so tests
// can hit both the prefix-write and line-write error paths independently.
type erroringWriter struct {
	calls     int
	failAfter int
}

func (w *erroringWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls > w.failAfter {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

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

func TestPrefixWriter_ReturnsErrorFromPrefixWrite(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //calls starts at zero, which is correct here
	w := logging.NewPrefixWriter("api", &erroringWriter{failAfter: 0})

	n, err := w.Write([]byte("boom\n"))

	assert.Error(t, err)
	assert.Equal(t, 5, n)
}

func TestPrefixWriter_ReturnsErrorFromLineWrite(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //calls starts at zero, which is correct here
	w := logging.NewPrefixWriter("api", &erroringWriter{failAfter: 1})

	n, err := w.Write([]byte("boom\n"))

	assert.Error(t, err)
	assert.Equal(t, 5, n)
}
