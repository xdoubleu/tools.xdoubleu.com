//nolint:mnd //no magic number
package config

import (
	"log/slog"

	"github.com/xdoubleu/essentia/v4/pkg/config"
)

type Config struct {
	Env             string
	Port            int
	Throttle        bool
	WebURL          string
	APIURL          string
	SentryDsn       string
	SampleRate      float64
	AccessExpiry    string
	RefreshExpiry   string
	DBDsn           string
	Release         string
	SupabaseProjRef string
	SupabaseAPIKey  string
	HardcoverAPIKey string
	SteamAPIKey     string
	R2AccountID     string
	R2AccessKeyID   string
	R2SecretKey     string
	R2Bucket        string
}

func New(logger *slog.Logger) Config {
	var cfg Config

	parser := config.New(logger)

	cfg.Env = parser.EnvStr("ENV", config.ProdEnv)
	cfg.Port = parser.EnvInt("PORT", 8000)
	cfg.Throttle = parser.EnvBool("THROTTLE", true)
	cfg.WebURL = parser.EnvStr("WEB_URL", "http://localhost:3000")
	cfg.APIURL = parser.EnvStr("API_URL", "http://localhost:8000")
	cfg.SentryDsn = parser.EnvStr("SENTRY_DSN", "")
	cfg.SampleRate = parser.EnvFloat("SAMPLE_RATE", 1.0)
	cfg.AccessExpiry = parser.EnvStr("ACCESS_EXPIRY", "1h")
	cfg.RefreshExpiry = parser.EnvStr("REFRESH_EXPIRY", "7d")
	cfg.DBDsn = parser.EnvStr("DB_DSN", "postgres://postgres@localhost/postgres")
	cfg.Release = parser.EnvStr("RELEASE", config.DevEnv)

	cfg.SupabaseProjRef = parser.EnvStr("SUPABASE_PROJ_REF", "")
	cfg.SupabaseAPIKey = parser.EnvStr("SUPABASE_API_KEY", "")

	cfg.HardcoverAPIKey = parser.EnvStr("HARDCOVER_API_KEY", "")
	cfg.SteamAPIKey = parser.EnvStr("STEAM_API_KEY", "")

	cfg.R2AccountID = parser.EnvStr("R2_ACCOUNT_ID", "")
	cfg.R2AccessKeyID = parser.EnvStr("R2_ACCESS_KEY_ID", "")
	cfg.R2SecretKey = parser.EnvStr("R2_SECRET_ACCESS_KEY", "")
	cfg.R2Bucket = parser.EnvStr("R2_BUCKET", "")

	return cfg
}
