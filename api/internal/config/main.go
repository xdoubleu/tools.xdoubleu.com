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

// ProdEnv is the production environment name.
const ProdEnv string = "production"

// TestEnv is the test environment name.
const TestEnv string = "test"

// DevEnv is the development environment name.
const DevEnv string = "development"

// RoleWeb serves requests and runs the light jobs; RoleWorkers runs only the
// memory-heavy jobs (books figure analysis, KEPUB conversion, trains import).
const (
	RoleWeb     string = "web"
	RoleWorkers string = "workers"
)

// IsWorkers reports whether this process runs the workers role.
func (c Config) IsWorkers() bool { return c.Role == RoleWorkers }

type Config struct {
	Env string
	// Role selects what this process runs: RoleWeb (default) or RoleWorkers.
	Role          string
	Port          int
	Throttle      bool
	WebURL        string
	APIURL        string
	SentryDsn     string
	SampleRate    float64
	AccessExpiry  string
	RefreshExpiry string
	AuthCacheTTL  int // seconds; 0 disables the per-token user cache
	DBDsn         string
	// DBMaxConns caps this process's pgx pool. Old and new containers of both
	// roles overlap during a deploy, so 2×(web+workers) must stay well under
	// Postgres' max_connections (100).
	DBMaxConns int
	Release    string
	// JWTSecret signs local session access-token JWTs.
	JWTSecret string
	// OAuthHMACSecret keys the embedded OAuth 2.1 server's HMAC token strategy.
	OAuthHMACSecret string
	// OAuthOIDCPrivateKey is a PEM RSA key signing OIDC ID tokens (public half at
	// /oauth2/jwks). Empty in dev: an ephemeral key is generated per restart.
	OAuthOIDCPrivateKey string
	// OAuthGrafanaClientSecret is the static "grafana" OAuth client's secret; its
	// bcrypt hash is reconciled on startup. Empty disables Grafana SSO.
	OAuthGrafanaClientSecret string
	// OAuthRoutinesClientSecret is the static "routines" machine client's
	// secret, reconciled like Grafana's. Empty disables the client.
	OAuthRoutinesClientSecret string
	// AuthIssuer is this api's OAuth 2.1 issuer URL. Defaults to APIURL.
	AuthIssuer      string
	SteamAPIKey     string
	HardcoverAPIKey string
	// TMDBAPIKey is a TMDB API Read Access Token. Empty disables movies search.
	TMDBAPIKey string
	// OpenRouterAPIKey powers the taste app's LLM stages; empty disables them.
	// TasteGeneratorModel writes the profile and proposes and scores picks
	// (docs/adr-0032-openrouter-llm-for-taste.md).
	OpenRouterAPIKey    string
	TasteGeneratorModel string

	R2AccountID   string
	R2AccessKeyID string
	R2SecretKey   string
	R2Bucket      string

	// App-level OAuth client credentials for the observability integrations; the
	// connections themselves live in global.oauth_connections.
	GithubOAuthClientID     string
	GithubOAuthClientSecret string
	SentryOAuthClientID     string
	SentryOAuthClientSecret string
	// TodoistOAuthClientID/Secret: per-user connections live in
	// learningpaths.oauth_connections.
	TodoistOAuthClientID     string
	TodoistOAuthClientSecret string
	// EncryptionKey is a base64 32-byte AES-256 key for OAuth tokens at rest.
	EncryptionKey string

	// Resend sender for notification emails; NotifyEmailTo is the admin address.
	ResendAPIKey  string
	EmailFrom     string
	NotifyEmailTo string

	// Email-relay newsletter feeds: EmailInboundDomain builds the
	// "reading+<token>@domain" alias, EmailInboundSecret verifies the Resend
	// webhook. Either empty disables the feature.
	EmailInboundDomain string
	EmailInboundSecret string

	// PrometheusURL is Prometheus's internal HTTP API base, used by prom_query.
	PrometheusURL string
	// GrafanaURL is Grafana's public base URL; Kamal gives containers no stable
	// alias, so the api goes through kamal-proxy.
	GrafanaURL string
	// GrafanaAdminPassword authenticates get_grafana_alerts as `admin`; empty
	// disables the tool.
	GrafanaAdminPassword string
	// ObservabilityIngestSecret is the bearer token GET /metrics requires, which
	// Prometheus sends; empty leaves /metrics open (the api is the root service,
	// so it is reachable through kamal-proxy). Never reuse it for user-facing auth.
	ObservabilityIngestSecret string

	// WebDir is web's build output (web/dist). Empty disables serving the web
	// app (local development runs the Vite dev server instead).
	WebDir string
	// SentryDsnWeb, PostHogKey and PostHogHost reach the browser through
	// /env.js; they ship in the bundle's runtime config anyway.
	SentryDsnWeb string
	PostHogKey   string
	PostHogHost  string
	// KoboGatewayRelease is the bundled kobo-gateway's release; it can lag
	// Release when its build cache-hit.
	KoboGatewayRelease string
	// OfflineDisabled serves a service worker that clears its caches and
	// unregisters itself.
	OfflineDisabled bool
}

type parser struct {
	logger *slog.Logger
}

const errorMessage = "can't convert env var '%s' with value '%s' to %s"

// newParser loads env vars, including from a .env file.
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

// envSecret is envStr that never logs the value.
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

// loadMonitoring reads the Prometheus and Grafana settings.
func (c parser) loadMonitoring(cfg *Config) {
	cfg.PrometheusURL = c.envStr(
		"PROMETHEUS_URL", "http://prometheus:9090",
	)
	cfg.GrafanaURL = c.envStr(
		"GRAFANA_URL", "https://tools.xdoubleu.com/grafana",
	)
	cfg.GrafanaAdminPassword = c.envSecret("GRAFANA_ADMIN_PASSWORD", "")
	cfg.ObservabilityIngestSecret = c.envSecret("OBSERVABILITY_INGEST_SECRET", "")
}

// loadWeb reads what the api needs to serve the web build.
func (c parser) loadWeb(cfg *Config) {
	cfg.WebDir = c.envStr("WEB_DIR", "")
	cfg.SentryDsnWeb = c.envSecret("SENTRY_DSN_WEB", "")
	cfg.PostHogKey = c.envSecret("POSTHOG_KEY", "")
	cfg.PostHogHost = c.envStr("POSTHOG_HOST", "")
	cfg.KoboGatewayRelease = c.envStr("KOBO_GATEWAY_RELEASE", "dev")
	cfg.OfflineDisabled = c.envStr("OFFLINE_DISABLED", "") == "1"
}

// dbMaxConnsDefault: workers runs two job-queue workers, web serves requests
// plus the light jobs (Steam sync fans out 10 at once).
func dbMaxConnsDefault(role string) int {
	if role == RoleWorkers {
		return 5
	}
	return 15
}

func (c parser) envRole() string {
	role := c.envStr("ROLE", RoleWeb)
	if role != RoleWeb && role != RoleWorkers {
		panic(fmt.Sprintf("invalid ROLE %q: want %q or %q", role, RoleWeb, RoleWorkers))
	}
	return role
}

func New(logger *slog.Logger) Config {
	var cfg Config

	p := newParser(logger)

	cfg.Env = p.envStr("ENV", ProdEnv)
	cfg.Role = p.envRole()
	cfg.Port = p.envInt("PORT", 8000)
	cfg.Throttle = p.envBool("THROTTLE", true)
	cfg.WebURL = p.envStr("WEB_URL", "http://localhost:3000")
	cfg.APIURL = p.envStr("API_URL", "http://localhost:8000")
	cfg.SentryDsn = p.envSecret("SENTRY_DSN", "")
	cfg.SampleRate = p.envFloat("SAMPLE_RATE", 1.0)
	cfg.AccessExpiry = p.envStr("ACCESS_EXPIRY", "1h")
	cfg.RefreshExpiry = p.envStr("REFRESH_EXPIRY", "7d")
	cfg.AuthCacheTTL = p.envInt("AUTH_CACHE_TTL", 60)
	cfg.DBDsn = p.envSecret("DB_DSN", "postgres://postgres@localhost/postgres")
	cfg.DBMaxConns = p.envInt("DB_MAX_CONNS", dbMaxConnsDefault(cfg.Role))
	// "dev" is the literal "no real deploy" sentinel web and kobo-gateway expect.
	cfg.Release = p.envStr("RELEASE", "dev")

	cfg.JWTSecret = p.envSecret("JWT_SECRET", "")
	cfg.OAuthHMACSecret = p.envSecret("OAUTH_HMAC_SECRET", "")
	cfg.OAuthOIDCPrivateKey = p.envSecret("OAUTH_OIDC_PRIVATE_KEY", "")
	cfg.OAuthGrafanaClientSecret = p.envSecret("OAUTH_GRAFANA_CLIENT_SECRET", "")
	cfg.OAuthRoutinesClientSecret = p.envSecret("OAUTH_ROUTINES_CLIENT_SECRET", "")
	cfg.AuthIssuer = p.envStr("AUTH_ISSUER", "")
	if cfg.AuthIssuer == "" {
		cfg.AuthIssuer = cfg.APIURL
	}

	cfg.SteamAPIKey = p.envSecret("STEAM_API_KEY", "")
	cfg.HardcoverAPIKey = p.envSecret("HARDCOVER_API_KEY", "")
	cfg.TMDBAPIKey = p.envSecret("TMDB_API_KEY", "")
	cfg.OpenRouterAPIKey = p.envSecret("OPENROUTER_API_KEY", "")
	cfg.TasteGeneratorModel = p.envStr(
		"TASTE_GENERATOR_MODEL", "mistralai/mistral-medium-3-5",
	)

	cfg.R2AccountID = p.envStr("R2_ACCOUNT_ID", "")
	cfg.R2AccessKeyID = p.envSecret("R2_ACCESS_KEY_ID", "")
	cfg.R2SecretKey = p.envSecret("R2_SECRET_ACCESS_KEY", "")
	cfg.R2Bucket = p.envStr("R2_BUCKET", "")

	cfg.GithubOAuthClientID = p.envStr("GITHUB_OAUTH_CLIENT_ID", "")
	cfg.GithubOAuthClientSecret = p.envSecret("GITHUB_OAUTH_CLIENT_SECRET", "")
	cfg.SentryOAuthClientID = p.envStr("SENTRY_OAUTH_CLIENT_ID", "")
	cfg.SentryOAuthClientSecret = p.envSecret("SENTRY_OAUTH_CLIENT_SECRET", "")
	cfg.TodoistOAuthClientID = p.envStr("TODOIST_OAUTH_CLIENT_ID", "")
	cfg.TodoistOAuthClientSecret = p.envSecret("TODOIST_OAUTH_CLIENT_SECRET", "")
	cfg.EncryptionKey = p.envSecret("ENCRYPTION_KEY", "")

	cfg.ResendAPIKey = p.envSecret("RESEND_API_KEY", "")
	cfg.EmailFrom = p.envStr("EMAIL_FROM", "")
	cfg.NotifyEmailTo = p.envStr("NOTIFY_EMAIL_TO", "")

	cfg.EmailInboundDomain = p.envStr("EMAIL_INBOUND_DOMAIN", "")
	cfg.EmailInboundSecret = p.envSecret("EMAIL_INBOUND_SECRET", "")

	p.loadMonitoring(&cfg)
	p.loadWeb(&cfg)

	return cfg
}
