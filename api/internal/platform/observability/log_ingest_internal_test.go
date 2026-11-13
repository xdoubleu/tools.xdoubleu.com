package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/kit/database/postgres"
	"tools.xdoubleu.com/internal/kit/logging"
	"tools.xdoubleu.com/internal/kit/testhelper"
	platformrepositories "tools.xdoubleu.com/internal/platform/repositories"
)

func newIngestDB(t *testing.T) postgres.DB {
	t.Helper()
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	t.Cleanup(pool.Close)
	return pool
}

func postLogs(t *testing.T, db postgres.DB, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewReader(body))
	WebLogsHandler(
		logging.NewNopLogger(),
		platformrepositories.NewLogsRepository(db),
	).ServeHTTP(rec, req)
	return rec
}

func clearWebClientLogs(t *testing.T, db postgres.DB) {
	t.Helper()
	_, err := db.Exec(
		context.Background(),
		"DELETE FROM global.log_entries WHERE source = 'web-client'",
	)
	require.NoError(t, err)
}

func TestWebLogsHandler_InvalidBody_BadRequest(t *testing.T) {
	db := newIngestDB(t)
	rec := postLogs(t, db, []byte(`not json`))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebLogsHandler_ValidBatch_StoresEntriesAsWebClient(t *testing.T) {
	db := newIngestDB(t)
	clearWebClientLogs(t, db)
	body, err := json.Marshal(ingestLogsRequest{
		Entries: []ingestLogEntry{
			//nolint:exhaustruct // Attrs intentionally omitted for this entry
			{OccurredAt: "", Level: "info", Message: "hello from the browser"},
			{
				OccurredAt: "2026-01-01T00:00:00Z",
				Level:      "error",
				Message:    "boom",
				Attrs:      json.RawMessage(`{"foo":"bar"}`),
			},
		},
	})
	require.NoError(t, err)

	rec := postLogs(t, db, body)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	var count int
	require.NoError(t, db.QueryRow(
		context.Background(),
		"SELECT count(*) FROM global.log_entries WHERE source = 'web-client'",
	).Scan(&count))
	assert.Equal(t, 2, count)
}

func TestWebLogsHandler_MalformedTimestamp_FallsBackToNow(t *testing.T) {
	db := newIngestDB(t)
	clearWebClientLogs(t, db)
	body, err := json.Marshal(ingestLogsRequest{
		Entries: []ingestLogEntry{
			//nolint:exhaustruct // Attrs intentionally omitted for this entry
			{OccurredAt: "not-a-timestamp", Level: "warn", Message: "bad clock"},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, postLogs(t, db, body).Code)

	var occurredAt time.Time
	require.NoError(t, db.QueryRow(
		context.Background(),
		`SELECT occurred_at FROM global.log_entries
		WHERE source = 'web-client' AND message = $1`,
		"bad clock",
	).Scan(&occurredAt))
	assert.WithinDuration(t, time.Now(), occurredAt, time.Minute)
}

func TestWebLogsHandler_TooManyEntries_Rejected(t *testing.T) {
	db := newIngestDB(t)
	body, err := json.Marshal(ingestLogsRequest{
		Entries: make([]ingestLogEntry, maxIngestEntries+1),
	})
	require.NoError(t, err)

	assert.Equal(t, http.StatusRequestEntityTooLarge, postLogs(t, db, body).Code)
}

func TestWebLogsHandler_OversizedBody_Rejected(t *testing.T) {
	db := newIngestDB(t)
	body := []byte(`{"entries":[{"message":"` +
		strings.Repeat("x", maxIngestBodyBytes) + `"}]}`)

	assert.Equal(t, http.StatusBadRequest, postLogs(t, db, body).Code)
}

// Browser-supplied entries are tagged, normalised and bounded.
func TestWebLogsHandler_SanitisesEntries(t *testing.T) {
	db := newIngestDB(t)
	clearWebClientLogs(t, db)
	body, err := json.Marshal(ingestLogsRequest{
		Entries: []ingestLogEntry{{
			OccurredAt: "2001-01-01T00:00:00Z",
			Level:      "critical",
			Message:    strings.Repeat("m", maxIngestMessageBytes+10),
			Attrs: json.RawMessage(
				`{"big":"` + strings.Repeat("a", maxIngestAttrsBytes) + `"}`,
			),
		}},
	})
	require.NoError(t, err)

	require.Equal(t, http.StatusNoContent, postLogs(t, db, body).Code)

	var (
		level      string
		message    string
		attrs      *string
		occurredAt time.Time
	)
	require.NoError(t, db.QueryRow(
		context.Background(), `
		SELECT level, message, attrs::text, occurred_at
		FROM global.log_entries WHERE source = 'web-client'`,
	).Scan(&level, &message, &attrs, &occurredAt))
	assert.Equal(t, "info", level)
	assert.Len(t, message, maxIngestMessageBytes)
	assert.Nil(t, attrs)
	assert.WithinDuration(t, time.Now(), occurredAt, time.Minute)
}

func TestTruncateUTF8(t *testing.T) {
	assert.Equal(t, "abc", truncateUTF8("abc", 5))
	assert.Equal(t, "ab", truncateUTF8("abc", 2))
	// "é" is two bytes; cutting inside it drops the whole rune.
	assert.Equal(t, "a", truncateUTF8("aé", 2))
}

func TestWebLogsHandler_StoreFails_InternalServerError(t *testing.T) {
	pool := testhelper.ConnectTestDB(testhelper.NewTestConfig().DBDsn)
	pool.Close()
	body, err := json.Marshal(ingestLogsRequest{
		Entries: []ingestLogEntry{
			{OccurredAt: "", Level: "info", Message: "x", Attrs: nil},
		},
	})
	require.NoError(t, err)

	rec := postLogs(t, pool, body)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
