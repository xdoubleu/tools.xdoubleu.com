package config_test

import (
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

func TestNewHonorsOverrides(t *testing.T) {
	t.Setenv("PORT", "9001")
	t.Setenv("THROTTLE", "false")
	cfg := config.New(logging.NewNopLogger())
	require.Equal(t, 9001, cfg.Port)
	assert.False(t, cfg.Throttle)
}
