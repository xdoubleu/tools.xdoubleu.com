package sentrytools_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/sentrytools"
)

func TestLogHandlerCapturesErrorWithGroupsAndAttrs(t *testing.T) {
	t.Parallel()

	hub := sentrytools.MockedSentryHub()
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, nil)
	handler := sentrytools.NewLogHandler("production", inner)

	logger := slog.New(handler).
		WithGroup("request").
		With(slog.String("route", "/x"))

	logger.ErrorContext(ctx, "boom", slog.Any("error", errors.New("db down")))

	events := sentrytools.MockedHubEvents(hub)
	require.Len(t, events, 1)
	assert.Equal(t, "db down", events[0].Exception[0].Value)
	assert.Equal(t, "/x", events[0].Tags["request.route"])
}

func TestLogHandlerIgnoresNonErrorLevels(t *testing.T) {
	t.Parallel()

	hub := sentrytools.MockedSentryHub()
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	var buf bytes.Buffer
	handler := sentrytools.NewLogHandler("production", slog.NewTextHandler(&buf, nil))
	slog.New(handler).InfoContext(ctx, "just info")

	assert.Empty(t, sentrytools.MockedHubEvents(hub))
}

func TestLogHandlerEnabled(t *testing.T) {
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
