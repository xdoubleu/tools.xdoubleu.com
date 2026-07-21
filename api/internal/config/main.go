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
	AuthCacheTTL    int // seconds; 0 disables the per-token user cache
	DBDsn           string
	Release         string
	SupabaseProjRef string
	SupabaseAPIKey  string
	SteamAPIKey     string
	HardcoverAPIKey string
	R2AccountID     string
	R2AccessKeyID   string
	R2SecretKey     string
	R2Bucket        string
	GithubRepo      string
	SentryOrg       string
	SentryProject   string
	DOAppID         string

	// OAuth app registration credentials for the observability integrations
	// (issue #440): each provider's connection itself is stored in
	// global.oauth_connections, not here — these are only the app's own
	// client id/secret, registered once with each provider.
	GithubOAuthClientID     string
	GithubOAuthClientSecret string
	SentryOAuthClientID     string
	SentryOAuthClientSecret string
	DOOAuthClientID         string
	DOOAuthClientSecret     string
	// OAuthTokenEncKey is a base64-standard-encoded 32-byte AES-256 key used
	// to encrypt stored OAuth tokens at rest (see internal/crypto).
	OAuthTokenEncKey string
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
	cfg.AuthCacheTTL = parser.EnvInt("AUTH_CACHE_TTL", 60)
	cfg.DBDsn = parser.EnvStr("DB_DSN", "postgres://postgres@localhost/postgres")
	cfg.Release = parser.EnvStr("RELEASE", config.DevEnv)

	cfg.SupabaseProjRef = parser.EnvStr("SUPABASE_PROJ_REF", "")
	cfg.SupabaseAPIKey = parser.EnvStr("SUPABASE_API_KEY", "")

	cfg.SteamAPIKey = parser.EnvStr("STEAM_API_KEY", "")
	cfg.HardcoverAPIKey = parser.EnvStr("HARDCOVER_API_KEY", "")

	cfg.R2AccountID = parser.EnvStr("R2_ACCOUNT_ID", "")
	cfg.R2AccessKeyID = parser.EnvStr("R2_ACCESS_KEY_ID", "")
	cfg.R2SecretKey = parser.EnvStr("R2_SECRET_ACCESS_KEY", "")
	cfg.R2Bucket = parser.EnvStr("R2_BUCKET", "")

	cfg.GithubRepo = parser.EnvStr("GITHUB_REPO", "")
	cfg.SentryOrg = parser.EnvStr("SENTRY_ORG", "")
	cfg.SentryProject = parser.EnvStr("SENTRY_PROJECT", "")
	cfg.DOAppID = parser.EnvStr("DO_APP_ID", "")

	cfg.GithubOAuthClientID = parser.EnvStr("GITHUB_OAUTH_CLIENT_ID", "")
	cfg.GithubOAuthClientSecret = parser.EnvStr("GITHUB_OAUTH_CLIENT_SECRET", "")
	cfg.SentryOAuthClientID = parser.EnvStr("SENTRY_OAUTH_CLIENT_ID", "")
	cfg.SentryOAuthClientSecret = parser.EnvStr("SENTRY_OAUTH_CLIENT_SECRET", "")
	cfg.DOOAuthClientID = parser.EnvStr("DO_OAUTH_CLIENT_ID", "")
	cfg.DOOAuthClientSecret = parser.EnvStr("DO_OAUTH_CLIENT_SECRET", "")
	cfg.OAuthTokenEncKey = parser.EnvStr("OAUTH_TOKEN_ENC_KEY", "")

	return cfg
}
