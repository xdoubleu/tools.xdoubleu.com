package mcptools

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveSlow serves body through logSlowRequests to a handler that reads the
// whole body, as the SDK does, and answers with contentType. It returns the
// logs and the body the handler read.
func serveSlow(
	t *testing.T, threshold time.Duration, contentType, body string,
) (string, string) {
	t.Helper()
	var logs bytes.Buffer
	var read []byte
	handler := logSlowRequests(slog.New(slog.NewTextHandler(&logs, nil)), threshold,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			read, err = io.ReadAll(r.Body)
			assert.NoError(t, err)
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			w.WriteHeader(http.StatusOK)
		}),
	)
	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/apps/mcp", strings.NewReader(body)))
	return logs.String(), string(read)
}

func TestLogSlowRequestsNamesMethodAndResponse(t *testing.T) {
	t.Parallel()
	body := `{"jsonrpc": "2.0", "id": 1, "method": "subscriptions/listen"}`

	logs, read := serveSlow(t, time.Nanosecond, "text/event-stream", body)

	assert.Equal(t, body, read, "the handler must get the whole body")
	assert.Equal(t, 1, strings.Count(logs, "slow MCP request"))
	assert.Contains(t, logs, "level=WARN")
	assert.Contains(t, logs, "methods=subscriptions/listen")
	assert.Contains(t, logs, "content_type=text/event-stream")
	assert.Contains(t, logs, "duration=")
}

// A batch names each message; a JSON-RPC response has no method.
func TestLogSlowRequestsNamesEveryBatchMessage(t *testing.T) {
	t.Parallel()
	body := `[{"jsonrpc": "2.0", "id": 1, "method": "ping"},
		{"jsonrpc": "2.0", "method": "notifications/initialized"},
		{"jsonrpc": "2.0", "id": 9, "result": {}}]`

	logs, _ := serveSlow(t, time.Nanosecond, "application/json", body)

	assert.Contains(t, logs, "methods=ping,notifications/initialized,response")
	assert.Contains(t, logs, "content_type=application/json")
}

func TestLogSlowRequestsUnparsedBody(t *testing.T) {
	t.Parallel()

	logs, read := serveSlow(t, time.Nanosecond, "", "not json")

	assert.Equal(t, "not json", read)
	assert.Contains(t, logs, "methods=unparsed")
	assert.Contains(t, logs, `content_type=""`)
}

func TestLogSlowRequestsQuietBelowThreshold(t *testing.T) {
	t.Parallel()

	logs, read := serveSlow(t, time.Hour, "text/event-stream", `{"method": "ping"}`)

	require.JSONEq(t, `{"method": "ping"}`, read)
	assert.Empty(t, logs)
}
