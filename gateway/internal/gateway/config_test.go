package gateway_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/gateway/internal/gateway"
	"tools.xdoubleu.com/gateway/internal/logging"
)

func TestNew_Defaults(t *testing.T) {
	cfg := gateway.New(logging.NewNopLogger())

	assert.Equal(t, "dev", cfg.Release)
	assert.Equal(t, "dev", cfg.KoboGatewayRelease)
	assert.Equal(t, 8000, cfg.Port)
	assert.Equal(t, 8001, cfg.APIPort)
	assert.Equal(t, 3000, cfg.WebPort)
}

func TestNew_ReadsEnvOverrides(t *testing.T) {
	t.Setenv("RELEASE", "abc1234")
	t.Setenv("KOBO_GATEWAY_RELEASE", "def5678")
	t.Setenv("PORT", "9000")
	t.Setenv("API_PORT", "9001")
	t.Setenv("WEB_PORT", "9002")

	cfg := gateway.New(logging.NewNopLogger())

	assert.Equal(t, "abc1234", cfg.Release)
	assert.Equal(t, "def5678", cfg.KoboGatewayRelease)
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, 9001, cfg.APIPort)
	assert.Equal(t, 9002, cfg.WebPort)
}

func TestNew_DoesNotLogSentryDsnValues(t *testing.T) {
	const dsn = "https://public@sentry.example/1"
	t.Setenv("SENTRY_DSN", dsn)
	t.Setenv("SENTRY_DSN_WEB", dsn)

	var buf bytes.Buffer
	cfg := gateway.New(slog.New(slog.NewTextHandler(&buf, nil)))

	assert.Equal(t, dsn, cfg.SentryDsn)
	assert.Equal(t, dsn, cfg.SentryDsnWeb)

	logs := buf.String()
	assert.NotContains(t, logs, dsn)
	assert.Contains(t, logs, "SENTRY_DSN'='<redacted>'")
	assert.Contains(t, logs, "SENTRY_DSN_WEB'='<redacted>'")
}
