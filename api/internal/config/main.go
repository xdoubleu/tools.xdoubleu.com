//nolint:mnd //no magic number
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"tools.xdoubleu.com/internal/convert"
)

// ProdEnv can be used as value when reading out the type of environment.
const ProdEnv string = "production"

// TestEnv can be used as value when reading out the type of environment.
const TestEnv string = "test"

// DevEnv can be used as value when reading out the type of environment.
const DevEnv string = "development"

type Config struct {
	Env             string
	Port            int
	Throttle        bool
	WebURL          string
	APIURL          string
	SentryDsn       string
	SentryDsnWeb    string
	SampleRate      float64
	AccessExpiry    string
	RefreshExpiry   string
	AuthCacheTTL    int // seconds; 0 disables the per-token user cache
	DBDsn           string
	Release         string
	SupabaseProjRef string
	SupabaseAPIKey  string
	// GoTrueURL overrides auth-go's default Supabase-hosted URL
	// (https://<SupabaseProjRef>.supabase.co/auth/v1) via WithCustomAuthURL,
	// pointing sign-in at self-hosted GoTrue instead. Empty keeps the
	// Supabase-hosted default.
	GoTrueURL string
	// JWTSecret signs the local access-token JWTs issued by internal/auth's
	// self-hosted implementation (issue #1039).
	JWTSecret string
	// OAuthHMACSecret keys the embedded OAuth 2.1 authorization server's
	// (internal/oauth2as, via ory/fosite) HMAC token strategy.
	OAuthHMACSecret string
	// AuthIssuer is this api's own OAuth 2.1 authorization-server issuer URL,
	// exposed via /.well-known/oauth-authorization-server and used as the
	// resource-server metadata's authorization server (issue #1039). Defaults
	// to APIURL.
	AuthIssuer      string
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

	// Email-relay newsletter feeds (issue #595): EmailInboundDomain is the
	// Resend receiving domain used to build a feed's "reading+<token>@domain"
	// inbound alias; EmailInboundSecret verifies the Resend inbound webhook's
	// signature. Either empty disables the feature (FeedService.CreateEmail
	// refuses, the webhook handler rejects all requests).
	EmailInboundDomain string
	EmailInboundSecret string
}

// parser extracts environment variables and parses them to the right type.
type parser struct {
	logger *slog.Logger
}

const errorMessage = "can't convert env var '%s' with value '%s' to %s"

// newParser loads environment variables that could be provided using a
// .env file (particularly useful during development).
func newParser(logger *slog.Logger) parser {
	_ = godotenv.Load()

	return parser{
		logger: logger,
	}
}

func (c parser) baseEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return ""
	}

	return value
}

func (c parser) logValue(valType string, key string, value any) {
	strVal, err := convert.AnyToString(value)
	if err != nil {
		panic(err)
	}

	c.logger.Info(
		fmt.Sprintf("loaded env var '%s'='%s' with type '%s'", key, strVal, valType),
	)
}

func (c parser) envStr(key string, defaultValue string) string {
	value := c.baseEnv(key)
	if len(value) == 0 {
		value = defaultValue
	}

	c.logValue("string", key, value)
	return value
}

// envSecret behaves like envStr but never logs the actual value, only
// whether one was set.
func (c parser) envSecret(key string, defaultValue string) string {
	value := c.baseEnv(key)
	if len(value) == 0 {
		value = defaultValue
	}

	masked := "<unset>"
	if len(value) > 0 {
		masked = "<redacted>"
	}
	c.logger.Info(
		fmt.Sprintf("loaded env var '%s'='%s' with type 'string'", key, masked),
	)
	return value
}

func (c parser) envInt(key string, defaultValue int) int {
	value := defaultValue

	strVal := c.baseEnv(key)
	if len(strVal) != 0 {
		intVal, err := strconv.Atoi(strVal)
		if err != nil {
			panic(fmt.Sprintf(errorMessage, key, strVal, "int"))
		}
		value = intVal
	}

	c.logValue("int", key, value)
	return value
}

func (c parser) envFloat(key string, defaultValue float64) float64 {
	value := defaultValue

	strVal := c.baseEnv(key)
	if len(strVal) != 0 {
		floatVal, err := strconv.ParseFloat(strVal, 64)
		if err != nil {
			panic(fmt.Sprintf(errorMessage, key, strVal, "float64"))
		}
		value = floatVal
	}

	c.logValue("float64", key, value)
	return value
}

func (c parser) envBool(key string, defaultValue bool) bool {
	value := defaultValue

	strVal := c.baseEnv(key)
	if len(strVal) != 0 {
		boolVal, err := strconv.ParseBool(strVal)
		if err != nil {
			panic(fmt.Sprintf(errorMessage, key, strVal, "bool"))
		}
		value = boolVal
	}

	c.logValue("bool", key, value)
	return value
}

func New(logger *slog.Logger) Config {
	var cfg Config

	p := newParser(logger)

	cfg.Env = p.envStr("ENV", ProdEnv)
	cfg.Port = p.envInt("PORT", 8000)
	cfg.Throttle = p.envBool("THROTTLE", true)
	cfg.WebURL = p.envStr("WEB_URL", "http://localhost:3000")
	cfg.APIURL = p.envStr("API_URL", "http://localhost:8000")
	cfg.SentryDsn = p.envSecret("SENTRY_DSN", "")
	cfg.SentryDsnWeb = p.envSecret("SENTRY_DSN_WEB", "")
	cfg.SampleRate = p.envFloat("SAMPLE_RATE", 1.0)
	cfg.AccessExpiry = p.envStr("ACCESS_EXPIRY", "1h")
	cfg.RefreshExpiry = p.envStr("REFRESH_EXPIRY", "7d")
	cfg.AuthCacheTTL = p.envInt("AUTH_CACHE_TTL", 60)
	cfg.DBDsn = p.envSecret("DB_DSN", "postgres://postgres@localhost/postgres")
	// "dev" (not DevEnv/"development") — web's getRelease() and the
	// kobo-gateway update-check both hardcode this exact literal as the
	// "no real deploy" sentinel (see web/lib/env.ts).
	cfg.Release = p.envStr("RELEASE", "dev")

	cfg.SupabaseProjRef = p.envStr("SUPABASE_PROJ_REF", "")
	cfg.SupabaseAPIKey = p.envSecret("SUPABASE_API_KEY", "")
	cfg.GoTrueURL = p.envStr("GOTRUE_URL", "")
	cfg.JWTSecret = p.envSecret("JWT_SECRET", "")
	cfg.OAuthHMACSecret = p.envSecret("OAUTH_HMAC_SECRET", "")
	cfg.AuthIssuer = p.envStr("AUTH_ISSUER", "")
	if cfg.AuthIssuer == "" {
		cfg.AuthIssuer = cfg.APIURL
	}

	cfg.SteamAPIKey = p.envSecret("STEAM_API_KEY", "")
	cfg.HardcoverAPIKey = p.envSecret("HARDCOVER_API_KEY", "")

	cfg.R2AccountID = p.envStr("R2_ACCOUNT_ID", "")
	cfg.R2AccessKeyID = p.envSecret("R2_ACCESS_KEY_ID", "")
	cfg.R2SecretKey = p.envSecret("R2_SECRET_ACCESS_KEY", "")
	cfg.R2Bucket = p.envStr("R2_BUCKET", "")

	cfg.GithubOAuthClientID = p.envStr("GITHUB_OAUTH_CLIENT_ID", "")
	cfg.GithubOAuthClientSecret = p.envSecret("GITHUB_OAUTH_CLIENT_SECRET", "")
	cfg.SentryOAuthClientID = p.envStr("SENTRY_OAUTH_CLIENT_ID", "")
	cfg.SentryOAuthClientSecret = p.envSecret("SENTRY_OAUTH_CLIENT_SECRET", "")
	cfg.DOOAuthClientID = p.envStr("DO_OAUTH_CLIENT_ID", "")
	cfg.DOOAuthClientSecret = p.envSecret("DO_OAUTH_CLIENT_SECRET", "")
	cfg.EncryptionKey = p.envSecret("ENCRYPTION_KEY", "")

	cfg.ResendAPIKey = p.envSecret("RESEND_API_KEY", "")
	cfg.EmailFrom = p.envStr("EMAIL_FROM", "")
	cfg.NotifyEmailTo = p.envStr("NOTIFY_EMAIL_TO", "")

	cfg.EmailInboundDomain = p.envStr("EMAIL_INBOUND_DOMAIN", "")
	cfg.EmailInboundSecret = p.envSecret("EMAIL_INBOUND_SECRET", "")

	return cfg
}
