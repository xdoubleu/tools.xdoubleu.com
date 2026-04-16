package logging

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// UserIDCarrier is a mutable pointer placed in context by the global
// request-logging middleware before auth runs. The auth middleware fills
// in the ID so the outer middleware can record the request afterwards.
type UserIDCarrier struct {
	ID string
}

// CarrierKey is the context key for *UserIDCarrier.
//
//nolint:gochecknoglobals // context key
var CarrierKey = struct{ name string }{"userIDCarrier"}

// UserIDContextKey is set to user.ID by the auth middleware on every
// authenticated request. The UserLogHandler reads it from context.
//
//nolint:gochecknoglobals // context key
var UserIDContextKey = struct{ name string }{"userID"}

// LogEntry is a single captured log or request record for a user.
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
}

// UserLogBuffer is a thread-safe per-user ring buffer of LogEntry values.
type UserLogBuffer struct {
	mu      sync.RWMutex
	entries map[string][]LogEntry
	maxSize int
}

// NewUserLogBuffer returns a buffer that keeps at most maxSize entries per user.
func NewUserLogBuffer(maxSize int) *UserLogBuffer {
	//nolint:exhaustruct // mu is intentionally zero-valued
	return &UserLogBuffer{
		entries: make(map[string][]LogEntry),
		maxSize: maxSize,
	}
}

// Record appends an entry for the given user, evicting the oldest if full.
func (b *UserLogBuffer) Record(userID string, e LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entries := b.entries[userID]
	if len(entries) >= b.maxSize {
		entries = entries[1:]
	}

	b.entries[userID] = append(entries, e)
}

// Get returns a copy of the entries for the given user (oldest first).
func (b *UserLogBuffer) Get(userID string) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	src := b.entries[userID]
	out := make([]LogEntry, len(src))
	copy(out, src)

	return out
}

// UserLogHandler wraps an existing slog.Handler and additionally writes every
// handled record into UserLogBuffer when the request context carries a user ID.
type UserLogHandler struct {
	inner  slog.Handler
	buffer *UserLogBuffer
}

// NewUserLogHandler returns a UserLogHandler wrapping inner.
func NewUserLogHandler(inner slog.Handler, buffer *UserLogBuffer) *UserLogHandler {
	return &UserLogHandler{inner: inner, buffer: buffer}
}

func (h *UserLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *UserLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := ctx.Value(UserIDContextKey).(string); ok && id != "" {
		h.buffer.Record(id, LogEntry{
			Time:    r.Time,
			Level:   r.Level.String(),
			Message: r.Message,
		})
	}

	return h.inner.Handle(ctx, r)
}

func (h *UserLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &UserLogHandler{inner: h.inner.WithAttrs(attrs), buffer: h.buffer}
}

func (h *UserLogHandler) WithGroup(name string) slog.Handler {
	return &UserLogHandler{inner: h.inner.WithGroup(name), buffer: h.buffer}
}
