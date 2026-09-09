package slack_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/slack"
)

func TestSendReturnsErrNotConfiguredWithoutURL(t *testing.T) {
	t.Parallel()

	err := slack.New().Send(t.Context(), "", "subject", "body")
	assert.ErrorIs(t, err, slack.ErrNotConfigured)
}

func TestSendPostsBlockKitPayload(t *testing.T) {
	t.Parallel()

	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	err := slack.New().Send(t.Context(), server.URL, "Alert fired", "the details")
	require.NoError(t, err)

	assert.Equal(t, "Alert fired", body["text"])
	blocks, ok := body["blocks"].([]any)
	require.True(t, ok)
	require.Len(t, blocks, 2)

	header, _ := blocks[0].(map[string]any)
	assert.Equal(t, "header", header["type"])

	section, _ := blocks[1].(map[string]any)
	sectionText, _ := section["text"].(map[string]any)
	assert.Equal(t, "mrkdwn", sectionText["type"])
	assert.Equal(t, "the details", sectionText["text"])
}

func TestSendTruncatesLongBody(t *testing.T) {
	t.Parallel()

	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	long := strings.Repeat("x", 5000)
	require.NoError(t, slack.New().Send(t.Context(), server.URL, "s", long))

	blocks, _ := body["blocks"].([]any)
	section, _ := blocks[1].(map[string]any)
	sectionText, _ := section["text"].(map[string]any)
	got, _ := sectionText["text"].(string)
	assert.Less(t, len([]rune(got)), 3000)
	assert.Contains(t, got, "truncated")
}

func TestSendReturnsErrorOnNonOKStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("invalid_payload"))
		},
	))
	defer server.Close()

	err := slack.New().Send(context.Background(), server.URL, "s", "b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "invalid_payload")
}
