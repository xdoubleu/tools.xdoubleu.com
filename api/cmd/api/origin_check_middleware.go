package main

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"tools.xdoubleu.com/internal/platform/oauth2as"
)

// originCheckMiddleware rejects browser-driven state-changing requests whose
// Origin (or Referer, when Origin is absent) is not one of allowedOrigins or
// the request's own host. Requests carrying neither header (non-browser
// clients) and Bearer requests pass, as do the OAuth endpoints that
// third-party clients call cross-origin.
func originCheckMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) ||
				strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") ||
				r.URL.Path == oauth2as.TokenPath ||
				r.URL.Path == oauth2as.RegisterPath {
				next.ServeHTTP(w, r)
				return
			}

			source := r.Header.Get("Origin")
			if source == "" {
				source = r.Header.Get("Referer")
			}
			if source != "" && !originAllowed(source, r.Host, allowedOrigins) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

func originAllowed(source, host string, allowed []string) bool {
	u, err := url.Parse(source)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Host == host {
		return true
	}
	origin := u.Scheme + "://" + u.Host
	return slices.Contains(allowed, origin)
}
