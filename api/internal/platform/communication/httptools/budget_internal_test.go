package httptools

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func never(*http.Request) bool { return false }

// A handler that outlives its budget is logged, so an overrun is visible
// even when the response it writes never arrives.
func TestBudgetLogsOverrun(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	h := budget(logger, never, time.Millisecond, time.Hour)(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		}),
	)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	assert.Contains(t, logs.String(), "request exceeded its budget")
	assert.Contains(t, logs.String(), "path=/x")
}

func TestBudgetQuietWithinBudget(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	h := budget(logger, never, time.Hour, time.Hour)(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Empty(t, logs.String())
}

// A long route whose writer can't move its write deadline is logged: it
// would be cut at the server's WriteTimeout.
func TestBudgetLogsUnmovableWriteDeadline(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	always := func(*http.Request) bool { return true }
	h := budget(logger, always, time.Hour, time.Hour)(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Contains(t, logs.String(), "cannot extend write deadline")
}
