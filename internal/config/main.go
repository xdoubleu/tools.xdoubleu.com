//nolint:mnd //no magic number
package config

import (
	"log/slog"

	"github.com/XDoubleU/essentia/pkg/config"
)

type Config struct {
	Env              string
	Port             int
	Throttle         bool
	WebURL           string
	SentryDsn        string
	SampleRate       float64
	AccessExpiry     string
	RefreshExpiry    string
	DBDsn            string
	Release          string
	SupabaseUserID   string
	SupabaseProjRef  string
	SupabaseAPIKey   string
	TodoistAPIKey    string
	TodoistProjectID string
	SteamAPIKey      string
	SteamUserID      string
	GoodreadsURL     string
}

func New(logger *slog.Logger) Config {
	var cfg Config

	parser := config.New(logger)

	cfg.Env = parser.EnvStr("ENV", config.ProdEnv)
	cfg.Port = parser.EnvInt("PORT", 8000)
	cfg.Throttle = parser.EnvBool("THROTTLE", true)
	cfg.WebURL = parser.EnvStr("WEB_URL", "http://localhost:8000")
	cfg.SentryDsn = parser.EnvStr("SENTRY_DSN", "")
	cfg.SampleRate = parser.EnvFloat("SAMPLE_RATE", 1.0)
	cfg.AccessExpiry = parser.EnvStr("ACCESS_EXPIRY", "1h")
	cfg.RefreshExpiry = parser.EnvStr("REFRESH_EXPIRY", "7d")
	cfg.DBDsn = parser.EnvStr("DB_DSN", "postgres://postgres@localhost/postgres")
	cfg.Release = parser.EnvStr("RELEASE", config.DevEnv)

	cfg.SupabaseUserID = parser.EnvStr("SUPABASE_USER_ID", "")
	cfg.SupabaseProjRef = parser.EnvStr("SUPABASE_PROJ_REF", "")
	cfg.SupabaseAPIKey = parser.EnvStr("SUPABASE_API_KEY", "")

	cfg.TodoistAPIKey = parser.EnvStr("TODOIST_API_KEY", "")
	cfg.TodoistProjectID = parser.EnvStr("TODOIST_PROJECT_ID", "")

	cfg.SteamAPIKey = parser.EnvStr("STEAM_API_KEY", "")
	cfg.SteamUserID = parser.EnvStr("STEAM_USER_ID", "")

	cfg.GoodreadsURL = parser.EnvStr("GOODREADS_URL", "")

	return cfg
}
