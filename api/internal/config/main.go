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
	// EncryptionKey is a base64-standard-encoded 32-byte AES-256 key used
	// to encrypt stored OAuth tokens at rest (see internal/crypto).
	EncryptionKey string

	// Resend credentials for the new-issue notification emails (issue #561):
	// ResendAPIKey/EmailFrom identify the sender, NotifyEmailTo is the single
	// admin address that gets notified.
	ResendAPIKey  string
	EmailFrom     string
	NotifyEmailTo string

	// WebEnabled starts the bundled Next.js standalone server as a child
	// process and front-doors it behind this binary (the merged single-
	// component deploy shape). false by default so local dev, tests and a
	// standalone api binary are unaffected — only the merged image sets it.
	WebEnabled  bool
	WebPort     int
	WebNodeBin  string
	WebServerJS string
	// SupabaseURL/SupabaseAnonKey are the browser-facing Supabase project
	// values the Next.js child needs; distinct from SupabaseProjRef/
	// SupabaseAPIKey above, which are the api's own GoTrue backend
	// credentials.
	SupabaseURL     string
	SupabaseAnonKey string

	// Email-relay newsletter feeds (issue #595): EmailInboundDomain is the
	// Resend receiving domain used to build a feed's "reading+<token>@domain"
	// inbound alias; EmailInboundSecret verifies the Resend inbound webhook's
	// signature. Either empty disables the feature (FeedService.CreateEmail
	// refuses, the webhook handler rejects all requests).
	EmailInboundDomain string
	EmailInboundSecret string
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

	cfg.GithubOAuthClientID = parser.EnvStr("GITHUB_OAUTH_CLIENT_ID", "")
	cfg.GithubOAuthClientSecret = parser.EnvStr("GITHUB_OAUTH_CLIENT_SECRET", "")
	cfg.SentryOAuthClientID = parser.EnvStr("SENTRY_OAUTH_CLIENT_ID", "")
	cfg.SentryOAuthClientSecret = parser.EnvStr("SENTRY_OAUTH_CLIENT_SECRET", "")
	cfg.DOOAuthClientID = parser.EnvStr("DO_OAUTH_CLIENT_ID", "")
	cfg.DOOAuthClientSecret = parser.EnvStr("DO_OAUTH_CLIENT_SECRET", "")
	cfg.EncryptionKey = parser.EnvStr("ENCRYPTION_KEY", "")

	cfg.ResendAPIKey = parser.EnvStr("RESEND_API_KEY", "")
	cfg.EmailFrom = parser.EnvStr("EMAIL_FROM", "")
	cfg.NotifyEmailTo = parser.EnvStr("NOTIFY_EMAIL_TO", "")

	cfg.WebEnabled = parser.EnvBool("WEB_ENABLED", false)
	cfg.WebPort = parser.EnvInt("WEB_PORT", 3000)
	cfg.WebNodeBin = parser.EnvStr("WEB_NODE_BIN", "node")
	cfg.WebServerJS = parser.EnvStr("WEB_SERVER_JS", "/app/web/server.js")
	cfg.SupabaseURL = parser.EnvStr("SUPABASE_URL", "")
	cfg.SupabaseAnonKey = parser.EnvStr("SUPABASE_ANON_KEY", "")

	cfg.EmailInboundDomain = parser.EnvStr("EMAIL_INBOUND_DOMAIN", "")
	cfg.EmailInboundSecret = parser.EnvStr("EMAIL_INBOUND_SECRET", "")

	return cfg
}
