package spa

import (
	"net/url"
	"regexp"
	"strings"
)

// gatewayURL is the kobo-gateway helper's loopback origin
// (web/lib/books/gatewayClient.ts).
const gatewayURL = "https://127.0.0.1:41132"

// postHogHostRE maps PostHog's ingestion host to its -assets sibling, which
// serves the config and extension scripts.
var postHogHostRE = regexp.MustCompile(`^https://([a-z-]+)\.i\.`)

// contentSecurityPolicy is the one policy for the HTML shell: the build has no
// inline scripts, so nothing needs a per-request nonce.
func contentSecurityPolicy(cfg Config) string {
	// R2: direct presigned upload PUTs. gatewayURL: the loopback kobo-gateway.
	connect := []string{
		"'self'", "*.sentry.io", "https://*.r2.cloudflarestorage.com", gatewayURL,
	}
	if origin := originOf(cfg.APIURL); origin != "" {
		connect = append(connect, origin)
	}

	script := []string{"'self'"}
	if cfg.PostHogHost != "" {
		assets := postHogHostRE.ReplaceAllString(cfg.PostHogHost, "https://$1-assets.i.")
		connect = append(connect, cfg.PostHogHost, assets)
		script = append(script, assets)
	}

	return strings.Join([]string{
		"default-src 'self'",
		"script-src " + strings.Join(script, " "),
		// blob:: EPUB stylesheets and fonts inside the Readium reader's frames.
		"style-src 'self' 'unsafe-inline' blob:",
		"font-src 'self' blob:",
		"img-src 'self' data: blob: https:",
		"connect-src " + strings.Join(connect, " "),
		// The Readium reader renders book sections into blob: iframes.
		"frame-src 'self' blob:",
		// The PDF reader's pdf.js worker.
		"worker-src 'self' blob:",
		// Not 'none': Readium's blob: frames inherit this CSP, and Safari enforces
		// it on them. 'self' still blocks cross-origin framing.
		"frame-ancestors 'self'",
		"base-uri 'self'",
		"form-action 'self'",
		"object-src 'none'",
	}, "; ")
}

func originOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
