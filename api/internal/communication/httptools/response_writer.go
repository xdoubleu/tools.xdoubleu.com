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
	status int
}

func (w *responseWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		panic(fmt.Errorf("ResponseWriter doesn't implement http.Flusher"))
	}

	flusher.Flush()
}

// ReadFrom uses the wrapped writer's ReadFrom when it has one, else Write.
func (w *responseWriter) ReadFrom(r io.Reader) (int64, error) {
	if reader, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		return reader.ReadFrom(r)
	}

	// Hide ReadFrom so io.Copy doesn't recurse back into this method.
	return io.Copy(struct{ io.Writer }{w.ResponseWriter}, r)
}

// NewResponseWriter returns a new [ResponseWriter].
func NewResponseWriter(w http.ResponseWriter) ResponseWriter {
	return &responseWriter{w, -1}
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status != -1 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w responseWriter) Status() int {
	return w.status
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}
