// Package logging contains helpers for logging.
package logging

import (
	"bytes"
	"io"
	"log/slog"
)

// ErrAttr returns a [slog.Attr] for err.
func ErrAttr(err error) slog.Attr {
	return slog.Any("error", err)
}

// NewNopLogger provides a NopLogger which uses [io.Discard] to write logs to.
func NewNopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// NewBufLogHandler returns a [slog.TextHandler] writing to buf.
func NewBufLogHandler(buf *bytes.Buffer, opts *slog.HandlerOptions) *slog.TextHandler {
	return slog.NewTextHandler(buf, opts)
}
