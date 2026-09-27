package main

import (
	"net/http"
	"strings"
)

// stripAPIPathPrefix strips /api in-process. kamal-proxy routes
// "/api,/.well-known" to this service without stripping, because
// /.well-known/* (OAuth discovery) must arrive unmodified and it can't strip
// only one prefix. Internal-only paths, reached over the Docker network
// without the prefix, are not served through it.
func stripAPIPathPrefix(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := strings.CutPrefix(r.URL.Path, "/api"); ok {
			if p == metricsPath {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = p
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
		}
		next.ServeHTTP(w, r)
	})
}
