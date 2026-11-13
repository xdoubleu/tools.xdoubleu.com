package httptools

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
)

// ResponseWriter captures the written status code.
type ResponseWriter interface {
	http.ResponseWriter
	http.Hijacker // need this for sentry
	http.Flusher  // need this for sentry
	io.ReaderFrom // need this for sentry
	Status() int
}

type responseWriter struct {
	http.ResponseWriter
	status int // 0 until a header is written
}

func (w *responseWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		panic(fmt.Errorf("ResponseWriter doesn't implement http.Flusher"))
	}

	w.implicitOK()
	flusher.Flush()
}

// Write records the implicit 200 when no header was written yet.
func (w *responseWriter) Write(b []byte) (int, error) {
	w.implicitOK()
	return w.ResponseWriter.Write(b)
}

// ReadFrom uses the wrapped writer's ReadFrom when it has one, else Write.
func (w *responseWriter) ReadFrom(r io.Reader) (int64, error) {
	w.implicitOK()

	if reader, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return reader.ReadFrom(r)
	}

	// Hide ReadFrom so io.Copy doesn't recurse back into this method.
	return io.Copy(struct{ io.Writer }{w.ResponseWriter}, r)
}

// NewResponseWriter returns a new [ResponseWriter].
func NewResponseWriter(w http.ResponseWriter) ResponseWriter {
	return &responseWriter{w, 0}
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// implicitOK locks in the 200 net/http sends when a body or flush comes
// before any WriteHeader, so a later WriteHeader can't change the status.
func (w *responseWriter) implicitOK() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
}

// Status is the status sent to the client. A handler that wrote nothing gets
// net/http's implicit 200.
func (w *responseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}
