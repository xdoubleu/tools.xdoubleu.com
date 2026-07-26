package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	essentialogger "github.com/xdoubleu/essentia/v4/pkg/logging"
)

// frontendProxy front-doors every request in the merged single-container
// deploy shape (issue #558), replicating the two DO App Platform ingress
// rules the separate api/web components used to get for free:
//
//   - GET /health goes to apiHandler directly, unstripped — DO's health
//     check hits the container port directly and web/app has no health
//     route.
//   - /api and /api/* go to apiHandler with the /api prefix stripped,
//     exactly like the ingress's `preserve_path_prefix: false` did — so
//     API_URL stays an absolute https://.../api URL and web/ needs no code
//     changes. This preserves an existing quirk on purpose: GET /api/version
//     is registered on the api mux itself (routes.go), so its external path
//     is /api/api/version, same as it is today.
//   - everything else (including web/app/release/route.ts, which
//     deliberately stays off /api) is proxied to the Next.js child process.
func (app *Application) frontendProxy(apiHandler http.Handler) http.Handler {
	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("127.0.0.1:%d", app.config.WebPort),
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		app.logger.Error(
			"web process unreachable",
			slog.String("path", r.URL.Path),
			essentialogger.ErrAttr(err),
		)
		http.Error(w, "web process unavailable", http.StatusServiceUnavailable)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == healthPath:
			apiHandler.ServeHTTP(w, r)
		case r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/"):
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
			apiHandler.ServeHTTP(w, r)
		default:
			proxy.ServeHTTP(w, r)
		}
	})
}
