package observability

import (
	"context"
	"encoding/json"
	"log/slog"

	"tools.xdoubleu.com/internal/models"
)

type logInserter interface {
	Insert(ctx context.Context, entry models.LogEntry) error
}

// LogRepoHandler tees every record into global.log_entries alongside the
// wrapped handler. Insert failures are dropped, never logged, to avoid a log
// storm when the DB is the problem.
type LogRepoHandler struct {
	next     slog.Handler
	inserter logInserter
}

// NewLogRepoHandler wraps next, inserting records as source "api".
func NewLogRepoHandler(next slog.Handler, inserter logInserter) *LogRepoHandler {
	return &LogRepoHandler{next: next, inserter: inserter}
}

func (h *LogRepoHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *LogRepoHandler) Handle(ctx context.Context, record slog.Record) error {
	h.insertBestEffort(record)
	return h.next.Handle(ctx, record)
}

func (h *LogRepoHandler) insertBestEffort(record slog.Record) {
	attrs := map[string]any{}
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	var attrsJSON []byte
	if len(attrs) > 0 {
		if marshaled, err := json.Marshal(attrs); err == nil {
			attrsJSON = marshaled
		}
	}

	// Background, not ctx: request contexts are often canceled by now.
	_ = h.inserter.Insert(context.Background(), models.LogEntry{
		OccurredAt: record.Time,
		Source:     "api",
		Level:      record.Level.String(),
		Message:    record.Message,
		AttrsJSON:  attrsJSON,
	})
}

func (h *LogRepoHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogRepoHandler{next: h.next.WithAttrs(attrs), inserter: h.inserter}
}

func (h *LogRepoHandler) WithGroup(name string) slog.Handler {
	return &LogRepoHandler{next: h.next.WithGroup(name), inserter: h.inserter}
}
