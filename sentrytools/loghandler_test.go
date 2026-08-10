package sentrytools_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/sentrytools"
)

// transportMock is a minimal in-memory [sentry.Transport] so tests can
// assert on the events LogHandler actually sends to Sentry.
type transportMock struct {
	mu     sync.Mutex
	events []*sentry.Event
}

func (t *transportMock) Configure(_ sentry.ClientOptions) {}
func (t *transportMock) Flush(_ time.Duration) bool       { return true }
func (t *transportMock) FlushWithContext(_ context.Context) bool {
	return true
}
func (t *transportMock) Close() {}

func (t *transportMock) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *transportMock) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.events
}

func mockedHub(t *testing.T) (*sentry.Hub, *transportMock) {
	t.Helper()

	transport := &transportMock{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "http://whatever@example.com/1337",
		Transport: transport,
	})
	require.NoError(t, err)

	return sentry.NewHub(client, sentry.NewScope()), transport
}

func TestNewLogHandler_Enabled(t *testing.T) {
	t.Parallel()

	prodHandler := sentrytools.NewLogHandler(
		"production",
		slog.NewTextHandler(nil, nil),
	)
	assert.False(t, prodHandler.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, prodHandler.Enabled(context.Background(), slog.LevelInfo))

	devHandler := sentrytools.NewLogHandler(
		"development",
		slog.NewTextHandler(nil, nil),
	)
	assert.True(t, devHandler.Enabled(context.Background(), slog.LevelDebug))
}

func TestLogHandler_IgnoresNonErrorLevels(t *testing.T) {
	t.Parallel()

	hub, transport := mockedHub(t)
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	var buf bytes.Buffer
	handler := sentrytools.NewLogHandler("production", slog.NewTextHandler(&buf, nil))
	slog.New(handler).InfoContext(ctx, "just info")

	assert.Empty(t, transport.Events())
}

func TestLogHandler_IgnoresErrorsWithoutHub(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	handler := sentrytools.NewLogHandler("production", slog.NewTextHandler(&buf, nil))
	slog.New(handler).Error("boom", slog.Any("error", errors.New("db down")))

	assert.Contains(t, buf.String(), "boom")
}

func TestLogHandler_CapturesErrorWithGroupsAndAttrs(t *testing.T) {
	t.Parallel()

	hub, transport := mockedHub(t)
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, nil)
	handler := sentrytools.NewLogHandler("production", inner)

	logger := slog.New(handler).
		WithGroup("request").
		With(slog.String("route", "/x"))

	logger.ErrorContext(ctx, "boom", slog.Any("error", errors.New("db down")))

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "db down", events[0].Exception[0].Value)
	assert.Equal(t, "/x", events[0].Tags["request.route"])
}

func TestLogHandler_CapturesErrorWithoutErrorAttr(t *testing.T) {
	t.Parallel()

	// A hub with a nil client is a safe no-op sink for CaptureException,
	// letting this exercise setRecordTags' fallback without a real transport.
	hub := sentry.NewHub(nil, sentry.NewScope())
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	var buf bytes.Buffer
	handler := sentrytools.NewLogHandler("production", slog.NewTextHandler(&buf, nil))

	assert.NotPanics(t, func() {
		slog.New(handler).ErrorContext(ctx, "boom without error attr")
	})
}
