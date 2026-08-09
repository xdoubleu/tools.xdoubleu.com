package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/xdoubleu/essentia/v4/pkg/communication/httptools"
	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
)

// healthPath is the api child's health-check route — routed unstripped,
// like /api/* is, rather than proxied to the web child. This is a routing
// *contract* between two independently deployed binaries (gateway and api),
// not something worth sharing a Go const for across the module boundary —
// api/cmd/api/health.go registers the same literal independently.
const healthPath = "/health"

// versionPath answers gateway's own release directly, unproxied — same
// shape as healthPath — so it stays available even when both children are
// unready and doesn't need either child's own version endpoint touched.
const versionPath = "/gateway/version"

// upstreamResponseHeaderTimeout caps how long a proxy target has to start
// sending response headers. A wedged child (alive, socket listening, event
// loop blocked) otherwise hangs the proxy forever with nothing logged;
// exceeding this fires ErrorHandler below (logged 503). See issue #634 (the
// original web-only version of this constant).
const upstreamResponseHeaderTimeout = 15 * time.Second

// NewHandler front-doors every request in the merged single-container
// deploy shape (issue #558), replicating the two DO App Platform ingress
// rules the separate api/web components used to get for free:
//
//   - GET /health goes to the api child directly, unstripped — DO's health
//     check hits the container port directly and web/app has no health
//     route.
//   - GET /gateway/version answers gateway's own release directly, same
//     shape as /health — not proxied to either child.
//   - /api and /api/* go to the api child with the /api prefix stripped,
//     exactly like the ingress's `preserve_path_prefix: false` did — so
//     API_URL stays an absolute https://.../api URL and web/ needs no code
//     changes. This preserves an existing quirk on purpose: GET /api/version
//     is registered on the api mux itself, so its external path is
//     /api/api/version, same as it is today.
//   - everything else (including web/app/release/route.ts, which
//     deliberately stays off /api) is proxied to the Next.js child process.
//
// Both targets can be legitimately not-ready-yet (the api child may still
// be running migrations, the web child may still be starting) — this isn't
// a new failure mode versus the old in-process version, just a new place
// the existing "upstream not ready" 503 can originate from, now that api
// isn't guaranteed fully started the instant the listener that fronts it
// starts.
func NewHandler(
	apiPort, webPort int, release string, logger *slog.Logger,
) http.Handler {
	apiProxy := newReverseProxy(apiPort, logger)
	webProxy := newReverseProxy(webPort, logger)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == healthPath:
			apiProxy.ServeHTTP(w, r)
		case r.URL.Path == versionPath:
			if err := httptools.WriteJSON(
				w, http.StatusOK, map[string]string{"release": release}, nil,
			); err != nil {
				logger.Error("failed to write version response", essentialogger.ErrAttr(err))
			}
		case r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/"):
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
			apiProxy.ServeHTTP(w, r)
		default:
			webProxy.ServeHTTP(w, r)
		}
	})
}

func newReverseProxy(port int, logger *slog.Logger) *httputil.ReverseProxy {
	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", port),
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Loopback target, so the zero-value Transport's defaults are fine — we
	// only need the ResponseHeaderTimeout cap.
	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: upstreamResponseHeaderTimeout,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error(
			"upstream process unreachable",
			slog.Int("port", port),
			slog.String("path", r.URL.Path),
			essentialogger.ErrAttr(err),
		)
		http.Error(w, "upstream process unavailable", http.StatusServiceUnavailable)
	}

	return proxy
}
