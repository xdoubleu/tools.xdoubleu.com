package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

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

// observabilityIngestRoute authenticates with the shared secret.
func (app *Application) observabilityIngestRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !app.observabilityIngestAuthorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req ingestLogsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
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
	for _, e := range entries {
		occurredAt := time.Now()
		if e.OccurredAt != "" {
			if parsed, err := time.Parse(time.RFC3339, e.OccurredAt); err == nil {
				occurredAt = parsed
			}
		}

		if err := app.logsRepo.Insert(r.Context(), models.LogEntry{
			OccurredAt: occurredAt,
			Source:     "web",
			Level:      e.Level,
			Message:    e.Message,
			AttrsJSON:  e.Attrs,
		}); err != nil {
			return err
		}
	}
	return nil
}
