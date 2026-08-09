package config_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"tools.xdoubleu.com/gateway/internal/config"
	"tools.xdoubleu.com/gateway/internal/logging"
)

func TestEnvStr(t *testing.T) {
	t.Setenv("GW_CFG_STR", "value")

	c := config.New(logging.NewNopLogger())

	assert.Equal(t, "value", c.EnvStr("GW_CFG_STR", "default"))
	assert.Equal(t, "default", c.EnvStr("GW_CFG_STR_MISSING", "default"))
}

func TestEnvInt(t *testing.T) {
	t.Setenv("GW_CFG_INT", "14")

	c := config.New(logging.NewNopLogger())

	assert.Equal(t, 14, c.EnvInt("GW_CFG_INT", 0))
	assert.Equal(t, 0, c.EnvInt("GW_CFG_INT_MISSING", 0))
}

func TestEnvIntPanicsOnInvalidValue(t *testing.T) {
	t.Setenv("GW_CFG_INT_BAD", "not-an-int")

	c := config.New(logging.NewNopLogger())

	assert.Panics(t, func() {
		c.EnvInt("GW_CFG_INT_BAD", 0)
	})
}

func TestEnvSecretDoesNotLogValue(t *testing.T) {
	const secret = "super-secret-dsn"
	t.Setenv("GW_CFG_SECRET", secret)

	var buf bytes.Buffer
	c := config.New(slog.New(slog.NewTextHandler(&buf, nil)))

	assert.Equal(t, secret, c.EnvSecret("GW_CFG_SECRET", ""))
	assert.Empty(t, c.EnvSecret("GW_CFG_SECRET_MISSING", ""))

	logs := buf.String()
	assert.NotContains(t, logs, secret)
	assert.Contains(t, logs, "GW_CFG_SECRET'='<redacted>'")
	assert.Contains(t, logs, "GW_CFG_SECRET_MISSING'='<unset>'")
}

func TestEnvConstants(t *testing.T) {
	assert.Equal(t, "production", config.ProdEnv)
	assert.Equal(t, "test", config.TestEnv)
	assert.Equal(t, "development", config.DevEnv)
}
