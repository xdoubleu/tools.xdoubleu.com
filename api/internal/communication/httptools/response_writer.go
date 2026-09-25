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

func (w *responseWriter) ReadFrom(r io.Reader) (int64, error) {
	reader, ok := w.ResponseWriter.(io.ReaderFrom)
	if !ok {
		panic(fmt.Errorf("ResponseWriter doesn't implement io.ReaderFrom"))
	}

	return reader.ReadFrom(r)
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
