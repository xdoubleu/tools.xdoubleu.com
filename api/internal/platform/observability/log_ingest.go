package observability

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"tools.xdoubleu.com/internal/models"
	platformrepositories "tools.xdoubleu.com/internal/platform/repositories"
)

// webLogSource tags entries the browser logger posts to /logs.
const webLogSource = "web-client"

// ingestLogEntry is one entry of a browser log batch.
type ingestLogEntry struct {
	OccurredAt string          `json:"occurred_at"` // RFC3339; empty means "now"
	Level      string          `json:"level"`
	Message    string          `json:"message"`
	Attrs      json.RawMessage `json:"attrs,omitempty"`
}

type ingestLogsRequest struct {
	Entries []ingestLogEntry `json:"entries"`
}

// Ingest bounds: the batches are browser-supplied and unauthenticated.
const (
	maxIngestBodyBytes    = 256 * 1024
	maxIngestEntries      = 100
	maxIngestMessageBytes = 4 * 1024
	maxIngestAttrsBytes   = 8 * 1024
	maxIngestClockSkew    = 10 * time.Minute
)

//nolint:gochecknoglobals //read-only allowlist
var ingestLevels = map[string]bool{
	"debug": true, "info": true, "warn": true, "error": true,
}

// WebLogsHandler stores the browser logger's batches (web/lib/logger.ts). The
// endpoint is public, so the SPA handler gates it by same-origin, body size and
// per-IP rate; it validates every entry before storing it.
func WebLogsHandler(
	logger *slog.Logger,
	logs *platformrepositories.LogsRepository,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ingestLogsRequest
		r.Body = http.MaxBytesReader(w, r.Body, maxIngestBodyBytes)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if len(req.Entries) > maxIngestEntries {
			http.Error(w, "too many entries", http.StatusRequestEntityTooLarge)
			return
		}

		if err := insertIngestedLogs(r, logs, req.Entries); err != nil {
			logger.ErrorContext(
				r.Context(), "failed to store ingested log entries",
				"error", err,
			)
			http.Error(w, "failed to store log entries", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

func insertIngestedLogs(
	r *http.Request,
	logs *platformrepositories.LogsRepository,
	entries []ingestLogEntry,
) error {
	for _, e := range entries {
		now := time.Now()
		occurredAt := now
		if parsed, err := time.Parse(time.RFC3339, e.OccurredAt); err == nil &&
			parsed.Sub(now).Abs() <= maxIngestClockSkew {
			occurredAt = parsed
		}

		level := e.Level
		if !ingestLevels[level] {
			level = "info"
		}
		attrs := e.Attrs
		if len(attrs) > maxIngestAttrsBytes {
			attrs = nil
		}

		if err := logs.Insert(r.Context(), models.LogEntry{
			OccurredAt: occurredAt,
			Source:     webLogSource,
			Level:      level,
			Message:    truncateUTF8(e.Message, maxIngestMessageBytes),
			AttrsJSON:  attrs,
		}); err != nil {
			return err
		}
	}
	return nil
}

// truncateUTF8 cuts s to at most n bytes without splitting a rune.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
