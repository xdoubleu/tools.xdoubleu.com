package gateway

import (
	"log/slog"

	"github.com/xdoubleu/essentia/v4/pkg/config"
)

// Config holds only what routing/process-supervision needs — a small
// subset of api's own config.Config, deliberately not shared across the
// module boundary (see this package's own CLAUDE.md).
type Config struct {
	Env       string
	SentryDsn string
	Release   string

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

	cfg.Port = parser.EnvInt("PORT", 8000)
	cfg.APIPort = parser.EnvInt("API_PORT", 8001)
	cfg.APIBinPath = parser.EnvStr("API_BIN_PATH", "/app/bin/api")

	cfg.WebPort = parser.EnvInt("WEB_PORT", 3000)
	cfg.WebNodeBin = parser.EnvStr("WEB_NODE_BIN", "node")
	cfg.WebServerJS = parser.EnvStr("WEB_SERVER_JS", "/app/web/server.js")

	cfg.APIURL = parser.EnvStr("API_URL", "http://localhost:8000")
	cfg.SentryDsnWeb = parser.EnvStr("SENTRY_DSN_WEB", "")
	cfg.SupabaseURL = parser.EnvStr("SUPABASE_URL", "")
	cfg.SupabaseAnonKey = parser.EnvStr("SUPABASE_ANON_KEY", "")

	return cfg
}
