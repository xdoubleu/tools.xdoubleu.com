package httptools_test

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/communication/httptools"
)

// hijackableRecorder wraps httptest.ResponseRecorder to also implement
// http.Hijacker and io.ReaderFrom, which the plain recorder doesn't.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
	readFrom bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	server, _ := net.Pipe()
	rw := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	return server, rw, nil
}

func (h *hijackableRecorder) ReadFrom(r io.Reader) (int64, error) {
	h.readFrom = true
	buf := make([]byte, 512)
	n, _ := r.Read(buf)
	return int64(n), nil
}

func TestResponseWriterHijack(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //hijacked/readFrom are zero-value, set by the calls under test
	inner := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := httptools.NewResponseWriter(inner)

	hijacker, ok := rw.(http.Hijacker)
	require.True(t, ok)

	_, _, err := hijacker.Hijack()
	require.NoError(t, err)
	assert.True(t, inner.hijacked)
}

func TestResponseWriterHijackUnsupported(t *testing.T) {
	t.Parallel()

	rw := httptools.NewResponseWriter(httptest.NewRecorder())
	hijacker, ok := rw.(http.Hijacker)
	require.True(t, ok)

	_, _, err := hijacker.Hijack()
	assert.True(t, errors.Is(err, err))
	require.Error(t, err)
}

func TestResponseWriterFlush(t *testing.T) {
	t.Parallel()

	inner := httptest.NewRecorder()
	rw := httptools.NewResponseWriter(inner)
	//nolint:forcetypeassert,errcheck //we control the type; recorder Flush never errors
	rw.(http.Flusher).Flush()

	assert.True(t, inner.Flushed)
}

func TestResponseWriterStatusAndWriteHeaderOnce(t *testing.T) {
	t.Parallel()

	inner := httptest.NewRecorder()
	rw := httptools.NewResponseWriter(inner)

	rw.WriteHeader(http.StatusTeapot)
	rw.WriteHeader(http.StatusOK) // ignored, status already set
	assert.Equal(t, http.StatusTeapot, rw.Status())
	assert.Equal(t, http.StatusTeapot, inner.Code)
}

func TestResponseWriterReadFrom(t *testing.T) {
	t.Parallel()

	//nolint:exhaustruct //hijacked/readFrom are zero-value, set by the calls under test
	inner := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	rw := httptools.NewResponseWriter(inner)

	n, err := rw.ReadFrom(strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Positive(t, n)
	assert.True(t, inner.readFrom)
}
