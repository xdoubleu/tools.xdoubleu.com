package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"
	"unicode/utf8"

	"tools.xdoubleu.com/internal/models"
)

// observabilityLogsIngestPath receives web's batched logs. web has no user
// session, so it authenticates with a shared-secret header, not Connect.
const observabilityLogsIngestPath = "/api/observability/logs"

// observabilityIngestSecretHeader carries OBSERVABILITY_INGEST_SECRET.
//
//nolint:gosec // this is a header name, not a credential
const observabilityIngestSecretHeader = "X-Observability-Ingest-Secret"

// ingestLogEntry is one entry of web's log batch.
type ingestLogEntry struct {
	OccurredAt string          `json:"occurred_at"` // RFC3339; empty means "now"
	Level      string          `json:"level"`
	Message    string          `json:"message"`
	Attrs      json.RawMessage `json:"attrs,omitempty"`
}

type ingestLogsRequest struct {
	Entries []ingestLogEntry `json:"entries"`
}

// Ingest bounds: web's relay forwards browser-supplied batches.
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

// ingestSource is web's server-side logger by default; its public relay
// marks browser-supplied entries web-client.
func ingestSource(r *http.Request) string {
	if r.URL.Query().Get("source") == "web-client" {
		return "web-client"
	}
	return "web"
}

// observabilityIngestRoute authenticates with the shared secret.
func (app *Application) observabilityIngestRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !app.observabilityIngestAuthorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

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

		if err := app.insertIngestedLogs(r, req.Entries); err != nil {
			app.logger.ErrorContext(
				r.Context(), "failed to store ingested log entries",
				"error", err,
			)
			http.Error(w, "failed to store log entries", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (app *Application) observabilityIngestAuthorized(r *http.Request) bool {
	if app.config.ObservabilityIngestSecret == "" {
		return false
	}
	provided := r.Header.Get(observabilityIngestSecretHeader)
	return subtle.ConstantTimeCompare(
		[]byte(provided), []byte(app.config.ObservabilityIngestSecret),
	) == 1
}

func (app *Application) insertIngestedLogs(
	r *http.Request, entries []ingestLogEntry,
) error {
	source := ingestSource(r)
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

		if err := app.logsRepo.Insert(r.Context(), models.LogEntry{
			OccurredAt: occurredAt,
			Source:     source,
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
