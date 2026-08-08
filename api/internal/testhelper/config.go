package testhelper

import (
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/logging"
)

// NewTestConfig returns the standard configuration for integration tests:
// test environment, throttling disabled, and a no-op logger for loading.
// App-specific overrides (API keys, etc.) are applied by the caller.
func NewTestConfig() config.Config {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = config.TestEnv
	cfg.Throttle = false
	// Disable the per-token auth cache so tests that mutate roles or app
	// access mid-run always observe fresh DB state.
	cfg.AuthCacheTTL = 0
	return cfg
}
