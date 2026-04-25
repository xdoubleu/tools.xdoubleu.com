//nolint:mnd //no magic number
package config

import (
	"encoding/base64"
	"log/slog"

	"github.com/xdoubleu/essentia/v3/pkg/config"
)

// devEncryptionKey is a well-known 32-byte key used only in dev/test environments.
// Production deployments must set ENCRYPTION_KEY to a securely generated value.
//
//nolint:gochecknoglobals //dev-only default, replaced by ENCRYPTION_KEY in prod
var devEncryptionKey = []byte(
	"dev-tools-xdoubleu-encrypt-key!!",
)

type Config struct {
	Env             string
	Port            int
	Throttle        bool
	WebURL          string
	SentryDsn       string
	SampleRate      float64
	AccessExpiry    string
	RefreshExpiry   string
	DBDsn           string
	Release         string
	SupabaseProjRef string
	SupabaseAPIKey  string
	GitHubToken     string
	GitHubRepo      string
	EncryptionKey   []byte
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

	cfg.SupabaseProjRef = parser.EnvStr("SUPABASE_PROJ_REF", "")
	cfg.SupabaseAPIKey = parser.EnvStr("SUPABASE_API_KEY", "")

	cfg.GitHubToken = parser.EnvStr("GITHUB_TOKEN", "")
	cfg.GitHubRepo = parser.EnvStr("GITHUB_REPO", "")

	encKeyStr := parser.EnvStr("ENCRYPTION_KEY", "")
	if encKeyStr == "" {
		cfg.EncryptionKey = devEncryptionKey
	} else {
		key, err := base64.StdEncoding.DecodeString(encKeyStr)
		if err != nil || len(key) != 32 {
			panic("ENCRYPTION_KEY must be a base64-encoded 32-byte value")
		}
		cfg.EncryptionKey = key
	}

	return cfg
}
