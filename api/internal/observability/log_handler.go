package observability

import (
	"context"
	"encoding/json"
	"log/slog"

	"tools.xdoubleu.com/internal/models"
)

// logInserter is the slice of *repositories.LogsRepository LogRepoHandler
// needs.
type logInserter interface {
	Insert(ctx context.Context, entry models.LogEntry) error
}

// LogRepoHandler tees every slog record into global.log_entries (via the
// injected inserter) in addition to forwarding it to the wrapped handler,
// so api's own logs are queryable from the same table web's forwarded logs
// land in (issue #1040). The DB write is best-effort: a failure is silently
// dropped rather than recursively logged, which would risk a log storm if
// the database itself is the problem.
type LogRepoHandler struct {
	next     slog.Handler
	inserter logInserter
}

// NewLogRepoHandler wraps next, inserting every record as source "api" via
// inserter before forwarding to next.
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

	// context.Background(), not ctx: a request context is often already
	// canceled by the time deferred logging runs, and a canceled context
	// would silently drop every log line's DB write.
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
