package sentrytools_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/gateway/internal/config"
	"tools.xdoubleu.com/gateway/internal/sentrytools"
)

func TestLogHandlerEnabled(t *testing.T) {
	t.Parallel()

	prodHandler := sentrytools.NewLogHandler(
		config.ProdEnv,
		slog.NewTextHandler(nil, nil),
	)
	assert.False(t, prodHandler.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, prodHandler.Enabled(context.Background(), slog.LevelInfo))

	devHandler := sentrytools.NewLogHandler(
		config.DevEnv,
		slog.NewTextHandler(nil, nil),
	)
	assert.True(t, devHandler.Enabled(context.Background(), slog.LevelDebug))
}

func TestLogHandlerIgnoresErrorsWithoutHub(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	handler := sentrytools.NewLogHandler(config.ProdEnv, slog.NewTextHandler(&buf, nil))
	slog.New(handler).Error("boom", slog.Any("error", errors.New("db down")))

	assert.Contains(t, buf.String(), "boom")
}

func TestLogHandlerCapturesErrorWithGroupsAndAttrsNoopClient(t *testing.T) {
	t.Parallel()

	// A hub with a nil client is a safe no-op sink for CaptureException,
	// letting this exercise Handle/setGoasTags/setRecordTags without a
	// real Sentry transport.
	hub := sentry.NewHub(nil, sentry.NewScope())
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	var buf bytes.Buffer
	handler := sentrytools.NewLogHandler(config.ProdEnv, slog.NewTextHandler(&buf, nil))

	logger := slog.New(handler).
		WithGroup("request").
		With(slog.String("route", "/x"))

	assert.NotPanics(t, func() {
		logger.ErrorContext(ctx, "boom", slog.Any("error", errors.New("db down")))
	})
	assert.Contains(t, buf.String(), "boom")
}

func TestLogHandlerCapturesErrorWithoutErrorAttr(t *testing.T) {
	t.Parallel()

	hub := sentry.NewHub(nil, sentry.NewScope())
	ctx := sentry.SetHubOnContext(context.Background(), hub)

	var buf bytes.Buffer
	handler := sentrytools.NewLogHandler(config.ProdEnv, slog.NewTextHandler(&buf, nil))

	assert.NotPanics(t, func() {
		slog.New(handler).ErrorContext(ctx, "boom without error attr")
	})
}
