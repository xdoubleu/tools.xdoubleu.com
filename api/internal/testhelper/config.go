package testhelper

import (
	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/logging"
)

// NewTestConfig returns the standard integration-test config; callers apply
// app-specific overrides.
func NewTestConfig() config.Config {
	cfg := config.New(logging.NewNopLogger())
	cfg.Env = config.TestEnv
	cfg.Throttle = false
	// Disable the auth cache so role/access changes are seen immediately.
	cfg.AuthCacheTTL = 0
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "test-jwt-secret-test-jwt-secret"
	}
	if cfg.OAuthHMACSecret == "" {
		cfg.OAuthHMACSecret = "test-oauth-hmac-secret-32-bytes!"
	}
	if cfg.EncryptionKey == "" {
		// 32 zero bytes: a valid crypto.Sealer key.
		cfg.EncryptionKey = "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA="
	}
	return cfg
}
