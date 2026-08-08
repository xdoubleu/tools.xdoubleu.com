package gateway

import (
	"context"
	"os"
	"os/exec"
	"strconv"
)

// NewAPICmd builds the api child's command. api is our own trusted binary
// (not a narrow-allowlist Node dependency like web below) so it gets the
// full parent environment — it needs DB/Supabase/OAuth/etc. secrets — with
// only PORT overridden so it doesn't collide with gateway's own listener.
func NewAPICmd(ctx context.Context, cfg Config) *exec.Cmd {
	//nolint:gosec // cfg.APIBinPath is server-owned config, not user input
	cmd := exec.CommandContext(ctx, cfg.APIBinPath)
	cmd.Env = apiProcessEnv(cfg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

func apiProcessEnv(cfg Config) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if len(e) >= 5 && e[:5] == "PORT=" {
			continue
		}
		env = append(env, e)
	}

	return append(env, "PORT="+strconv.Itoa(cfg.APIPort))
}

// NewWebCmd builds the Next.js standalone server child's command. Unlike
// api above, this is a Node dependency we don't fully trust with our
// secrets, so its env is an explicit allowlist rather than the full parent
// environment.
func NewWebCmd(ctx context.Context, cfg Config) *exec.Cmd {
	//nolint:gosec // WebNodeBin/WebServerJS are server-owned config, not user input
	cmd := exec.CommandContext(ctx, cfg.WebNodeBin, cfg.WebServerJS)
	cmd.Env = webProcessEnv(cfg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd
}

// webProcessEnv builds an explicit allowlist for the child's environment
// rather than passing os.Environ() wholesale. RELEASE must always reach it
// — without it getRelease() returns 'dev' in the browser and
// gatewayNeedsUpdate (web/lib/books/gatewayClient.ts) silently stops
// detecting installed kobo-gateway updates for every user.
func webProcessEnv(cfg Config) []string {
	return []string{
		"PORT=" + strconv.Itoa(cfg.WebPort),
		"HOSTNAME=127.0.0.1",
		"NODE_ENV=production",
		"NODE_OPTIONS=--max-old-space-size=192",
		"API_URL=" + cfg.APIURL,
		"SENTRY_DSN=" + cfg.SentryDsnWeb,
		"SUPABASE_URL=" + cfg.SupabaseURL,
		"SUPABASE_ANON_KEY=" + cfg.SupabaseAnonKey,
		"RELEASE=" + cfg.Release,
	}
}
