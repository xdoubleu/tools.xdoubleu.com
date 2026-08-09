package logging

import (
	"bytes"
	"io"
)

// PrefixWriter wraps dst, prefixing every line written to it with
// "[name] " so the merged container's single stdout/stderr stream can tell
// gateway's own output apart from its api/web children's. Each child's
// stdout and stderr get their own PrefixWriter (see gateway/children.go),
// and each is only ever written to by one os/exec copy goroutine, so no
// locking is needed here.
type PrefixWriter struct {
	prefix []byte
	dst    io.Writer
	buf    []byte
}

// NewPrefixWriter returns a PrefixWriter tagging every line written to it
// with name.
func NewPrefixWriter(name string, dst io.Writer) *PrefixWriter {
	//nolint:exhaustruct //buf starts nil/empty, which is correct here
	return &PrefixWriter{prefix: []byte("[" + name + "] "), dst: dst}
}

// Write buffers p and flushes only complete lines (ending in '\n') to dst,
// each prefixed with the configured name. A trailing partial line is held
// back until the next Write completes it.
//
// ponytail: a final partial line with no trailing newline is never flushed;
// add a Flush/Close if a child ever writes unterminated output on exit.
func (w *PrefixWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)

	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}

		line := w.buf[:i+1]
		w.buf = w.buf[i+1:]

		if _, err := w.dst.Write(w.prefix); err != nil {
			return len(p), err
		}
		if _, err := w.dst.Write(line); err != nil {
			return len(p), err
		}
	}

	return len(p), nil
}
