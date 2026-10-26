package config_test

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/logging"
)

func TestNewDefaults(t *testing.T) {
	cfg := config.New(logging.NewNopLogger())
	assert.Equal(t, config.ProdEnv, cfg.Env)
	assert.Equal(t, 8000, cfg.Port)
	assert.True(t, cfg.Throttle)
	assert.Equal(t, config.RoleWeb, cfg.Role)
}

func TestNewRoleWorkers(t *testing.T) {
	t.Setenv("ROLE", config.RoleWorkers)
	assert.Equal(t, config.RoleWorkers, config.New(logging.NewNopLogger()).Role)
}

func TestNewDBMaxConnsDefaultsPerRole(t *testing.T) {
	assert.Equal(t, 15, config.New(logging.NewNopLogger()).DBMaxConns)

	t.Setenv("ROLE", config.RoleWorkers)
	assert.Equal(t, 5, config.New(logging.NewNopLogger()).DBMaxConns)

	t.Setenv("DB_MAX_CONNS", "8")
	assert.Equal(t, 8, config.New(logging.NewNopLogger()).DBMaxConns)
}

func TestNewPanicsOnInvalidRole(t *testing.T) {
	t.Setenv("ROLE", "nope")
	assert.Panics(t, func() { config.New(logging.NewNopLogger()) })
}

func TestNewPanicsOnInvalidInt(t *testing.T) {
	t.Setenv("PORT", "not-an-int")
	assert.Panics(t, func() { config.New(logging.NewNopLogger()) })
}

func TestNewPanicsOnInvalidFloat(t *testing.T) {
	t.Setenv("SAMPLE_RATE", "not-a-float")
	assert.Panics(t, func() { config.New(logging.NewNopLogger()) })
}

func TestNewPanicsOnInvalidBool(t *testing.T) {
	t.Setenv("THROTTLE", "not-a-bool")
	assert.Panics(t, func() { config.New(logging.NewNopLogger()) })
}

func TestNewDoesNotLogSecretValues(t *testing.T) {
	const secret = "super-secret-db-password"
	t.Setenv("DB_DSN", secret)
	t.Setenv("ENCRYPTION_KEY", secret)

	var buf bytes.Buffer
	logger := slog.New(logging.NewBufLogHandler(&buf, nil))
	config.New(logger)

	logs := buf.String()
	assert.NotContains(t, logs, secret)
	assert.Contains(t, logs, "DB_DSN'='<redacted>'")
	assert.Contains(t, logs, "ENCRYPTION_KEY'='<redacted>'")
}

func TestNewLogsLoadedValuesByType(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("PORT", "9001")
	t.Setenv("SAMPLE_RATE", "0.5")
	t.Setenv("THROTTLE", "false")

	var buf bytes.Buffer
	config.New(slog.New(logging.NewBufLogHandler(&buf, nil)))

	logs := buf.String()
	assert.Contains(t, logs, "'ENV'='development' with type 'string'")
	assert.Contains(t, logs, "'PORT'='9001' with type 'int'")
	assert.Contains(t, logs, "'SAMPLE_RATE'='0.50' with type 'float64'")
	assert.Contains(t, logs, "'THROTTLE'='false' with type 'bool'")
}

func TestNewHonorsOverrides(t *testing.T) {
	t.Setenv("PORT", "9001")
	t.Setenv("THROTTLE", "false")
	cfg := config.New(logging.NewNopLogger())
	require.Equal(t, 9001, cfg.Port)
	assert.False(t, cfg.Throttle)
}
