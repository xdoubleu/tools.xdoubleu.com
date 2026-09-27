package main

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
)

func withIngestSecret(t *testing.T, secret string) {
	t.Helper()
	original := testApp.config.ObservabilityIngestSecret
	testApp.config.ObservabilityIngestSecret = secret
	t.Cleanup(func() { testApp.config.ObservabilityIngestSecret = original })
}

func TestObservabilityIngestRoute_MissingSecret_Unauthorized(t *testing.T) {
	withIngestSecret(t, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, observabilityLogsIngestPath, bytes.NewBufferString(`{}`),
	)
	testApp.observabilityIngestRoute()(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestObservabilityIngestRoute_WrongSecret_Unauthorized(t *testing.T) {
	withIngestSecret(t, "correct-secret")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, observabilityLogsIngestPath, bytes.NewBufferString(`{}`),
	)
	req.Header.Set(observabilityIngestSecretHeader, "wrong-secret")
	testApp.observabilityIngestRoute()(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestObservabilityIngestRoute_InvalidBody_BadRequest(t *testing.T) {
	withIngestSecret(t, "correct-secret")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, observabilityLogsIngestPath, bytes.NewBufferString(`not json`),
	)
	req.Header.Set(observabilityIngestSecretHeader, "correct-secret")
	testApp.observabilityIngestRoute()(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestObservabilityIngestRoute_ValidBatch_StoresEntries(t *testing.T) {
	withIngestSecret(t, "correct-secret")
	_, err := testApp.db.Exec(
		context.Background(), "DELETE FROM global.log_entries WHERE source = 'web'",
	)
	require.NoError(t, err)

	body, err := json.Marshal(ingestLogsRequest{
		Entries: []ingestLogEntry{
			//nolint:exhaustruct // Attrs intentionally omitted for this entry
			{OccurredAt: "", Level: "info", Message: "hello from web"},
			{
				OccurredAt: "2026-01-01T00:00:00Z",
				Level:      "error",
				Message:    "boom",
				Attrs:      json.RawMessage(`{"foo":"bar"}`),
			},
		},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, observabilityLogsIngestPath, bytes.NewReader(body),
	)
	req.Header.Set(observabilityIngestSecretHeader, "correct-secret")
	testApp.observabilityIngestRoute()(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	var count int
	err = testApp.db.QueryRow(
		context.Background(),
		"SELECT count(*) FROM global.log_entries WHERE source = 'web'",
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestObservabilityIngestRoute_MalformedTimestamp_FallsBackToNow(t *testing.T) {
	withIngestSecret(t, "correct-secret")
	_, err := testApp.db.Exec(
		context.Background(), "DELETE FROM global.log_entries WHERE source = 'web'",
	)
	require.NoError(t, err)

	body, err := json.Marshal(ingestLogsRequest{
		Entries: []ingestLogEntry{
			//nolint:exhaustruct // Attrs intentionally omitted for this entry
			{OccurredAt: "not-a-timestamp", Level: "warn", Message: "bad clock"},
		},
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost, observabilityLogsIngestPath, bytes.NewReader(body),
	)
	req.Header.Set(observabilityIngestSecretHeader, "correct-secret")
	testApp.observabilityIngestRoute()(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	var occurredAt time.Time
	err = testApp.db.QueryRow(
		context.Background(),
		"SELECT occurred_at FROM global.log_entries WHERE source = 'web' AND message = $1",
		"bad clock",
	).Scan(&occurredAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), occurredAt, time.Minute)
}

func TestObservabilityIngestAuthorized(t *testing.T) {
	withIngestSecret(t, "shhh")

	req := httptest.NewRequest(http.MethodPost, observabilityLogsIngestPath, nil)
	assert.False(t, testApp.observabilityIngestAuthorized(req))

	req.Header.Set(observabilityIngestSecretHeader, "shhh")
	assert.True(t, testApp.observabilityIngestAuthorized(req))
}

func postIngest(t *testing.T, path string, body []byte) int {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set(observabilityIngestSecretHeader, "correct-secret")
	testApp.observabilityIngestRoute()(rec, req)
	return rec.Code
}

func TestObservabilityIngestRoute_TooManyEntries_Rejected(t *testing.T) {
	withIngestSecret(t, "correct-secret")
	entries := make([]ingestLogEntry, maxIngestEntries+1)
	body, err := json.Marshal(ingestLogsRequest{Entries: entries})
	require.NoError(t, err)

	assert.Equal(
		t, http.StatusRequestEntityTooLarge,
		postIngest(t, observabilityLogsIngestPath, body),
	)
}

func TestObservabilityIngestRoute_OversizedBody_Rejected(t *testing.T) {
	withIngestSecret(t, "correct-secret")
	body := []byte(`{"entries":[{"message":"` +
		strings.Repeat("x", maxIngestBodyBytes) + `"}]}`)

	assert.Equal(
		t, http.StatusBadRequest, postIngest(t, observabilityLogsIngestPath, body),
	)
}

// Browser-relayed entries are tagged, normalised and bounded.
func TestObservabilityIngestRoute_WebClient_SanitisesEntries(t *testing.T) {
	withIngestSecret(t, "correct-secret")
	_, err := testApp.db.Exec(
		context.Background(),
		"DELETE FROM global.log_entries WHERE source = 'web-client'",
	)
	require.NoError(t, err)

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

	require.Equal(t, http.StatusNoContent, postIngest(
		t, observabilityLogsIngestPath+"?source=web-client", body,
	))

	var (
		level      string
		message    string
		attrs      *string
		occurredAt time.Time
	)
	require.NoError(t, testApp.db.QueryRow(
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
