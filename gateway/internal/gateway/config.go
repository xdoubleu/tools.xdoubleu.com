package gateway

import (
	"log/slog"

	"github.com/xdoubleu/essentia/v4/pkg/config"
)

const (
	defaultPort    = 8000
	defaultAPIPort = 8001
	defaultWebPort = 3000
)

// Config holds only what routing/process-supervision needs — a small
// subset of api's own config.Config, deliberately not shared across the
// module boundary (see this package's own CLAUDE.md).
type Config struct {
	Env       string
	SentryDsn string
	Release   string

	// KoboGatewayRelease is the release actually baked into the bundled
	// kobo-gateway .dmg/binary — can lag behind Release when kobo-gateway's
	// own build was skipped (unchanged source, see build-kobo-gateway.yml).
	// Forwarded to the web child so gatewayNeedsUpdate compares against the
	// artifact that's actually bundled, not this deploy's overall SHA.
	KoboGatewayRelease string

	// Port is external — the only thing DO's edge/health check ever hits.
	Port int

	// APIPort/APIBinPath: the api child listens on APIPort instead of its
	// own default Port env, and is spawned from APIBinPath.
	APIPort    int
	APIBinPath string

	// WebPort/WebNodeBin/WebServerJS: unchanged defaults from the old
	// cmd/api-owned web process.
	WebPort     int
	WebNodeBin  string
	WebServerJS string

	// Forwarded into the web child's env exactly as before.
	APIURL          string
	SentryDsnWeb    string
	SupabaseURL     string
	SupabaseAnonKey string
}

func New(logger *slog.Logger) Config {
	var cfg Config

	parser := config.New(logger)

	cfg.Env = parser.EnvStr("ENV", config.ProdEnv)
	cfg.SentryDsn = parser.EnvStr("SENTRY_DSN", "")
	cfg.Release = parser.EnvStr("RELEASE", config.DevEnv)
	cfg.KoboGatewayRelease = parser.EnvStr("KOBO_GATEWAY_RELEASE", config.DevEnv)

	cfg.Port = parser.EnvInt("PORT", defaultPort)
	cfg.APIPort = parser.EnvInt("API_PORT", defaultAPIPort)
	cfg.APIBinPath = parser.EnvStr("API_BIN_PATH", "/app/bin/api")

	cfg.WebPort = parser.EnvInt("WEB_PORT", defaultWebPort)
	cfg.WebNodeBin = parser.EnvStr("WEB_NODE_BIN", "node")
	cfg.WebServerJS = parser.EnvStr("WEB_SERVER_JS", "/app/web/server.js")

	cfg.APIURL = parser.EnvStr("API_URL", "http://localhost:8000")
	cfg.SentryDsnWeb = parser.EnvStr("SENTRY_DSN_WEB", "")
	cfg.SupabaseURL = parser.EnvStr("SUPABASE_URL", "")
	cfg.SupabaseAnonKey = parser.EnvStr("SUPABASE_ANON_KEY", "")

	return cfg
}
