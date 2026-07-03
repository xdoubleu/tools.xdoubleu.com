package testhelper

import (
	configtools "github.com/xdoubleu/essentia/v4/pkg/config"
	"github.com/xdoubleu/essentia/v4/pkg/logging"

	"tools.xdoubleu.com/internal/config"
)

// NewTestConfig returns the standard configuration for integration tests:
// test environment, throttling disabled, and a no-op logger for loading.
// App-specific overrides (API keys, etc.) are applied by the caller.
func NewTestConfig() config.Config {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = configtools.TestEnv
	cfg.Throttle = false
	return cfg
}
