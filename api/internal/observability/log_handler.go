package observability

import (
	"context"
	"encoding/json"
	"log/slog"

	"tools.xdoubleu.com/internal/models"
)

// LogRepoHandler tees every record into global.log_entries alongside the
// wrapped handler, through an async batching writer (logWriter). Records the
// writer can't queue or insert are counted in log_entries_dropped_total,
// never logged, to avoid a log storm when the DB is the problem.
type LogRepoHandler struct {
	next   slog.Handler
	writer *logWriter
}

// NewLogRepoHandler wraps next, inserting records as source "api". Call Close
// on shutdown to write what is still queued.
func NewLogRepoHandler(
	next slog.Handler,
	inserter logBatchInserter,
) *LogRepoHandler {
	return newLogRepoHandler(next, inserter, defaultLogWriterOptions())
}

func newLogRepoHandler(
	next slog.Handler,
	inserter logBatchInserter,
	opts logWriterOptions,
) *LogRepoHandler {
	return &LogRepoHandler{next: next, writer: newLogWriter(inserter, opts)}
}

// Close writes the queued records, giving up when ctx ends. Idempotent.
func (h *LogRepoHandler) Close(ctx context.Context) error {
	return h.writer.Close(ctx)
}

func (h *LogRepoHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *LogRepoHandler) Handle(ctx context.Context, record slog.Record) error {
	h.writer.enqueue(logEntry(record))
	return h.next.Handle(ctx, record)
}

func logEntry(record slog.Record) models.LogEntry {
	attrs := map[string]any{}
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = jsonValue(a.Value)
		return true
	})

	var attrsJSON []byte
	if len(attrs) > 0 {
		if marshaled, err := json.Marshal(attrs); err == nil {
			attrsJSON = marshaled
		}
	}

	return models.LogEntry{
		OccurredAt: record.Time,
		Source:     "api",
		Level:      record.Level.String(),
		Message:    record.Message,
		AttrsJSON:  attrsJSON,
	}
}

// jsonValue converts v into something json.Marshal renders faithfully: errors
// marshal to {} and groups to Key/Value pairs otherwise.
func jsonValue(v slog.Value) any {
	v = v.Resolve()
	if v.Kind() == slog.KindGroup {
		group := map[string]any{}
		for _, a := range v.Group() {
			group[a.Key] = jsonValue(a.Value)
		}
		return group
	}
	if err, ok := v.Any().(error); ok {
		return err.Error()
	}
	return v.Any()
}

func (h *LogRepoHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LogRepoHandler{next: h.next.WithAttrs(attrs), writer: h.writer}
}

func (h *LogRepoHandler) WithGroup(name string) slog.Handler {
	return &LogRepoHandler{next: h.next.WithGroup(name), writer: h.writer}
}
