package main

import (
	"net/http"
	"strings"
)

// stripAPIPathPrefix replaces the /api-stripping half of what gateway/ used
// to do before it was retired (#1038): kamal-proxy's path_prefix for this
// service is "/api,/.well-known" with strip_path_prefix disabled, since
// /.well-known/* (RFC 9728/8414 OAuth discovery) must reach this process
// unmodified and kamal-proxy can't strip only one of several prefixes on a
// single service. So this middleware does the /api stripping on its own,
// leaving every other path (including /.well-known/*) untouched.
func stripAPIPathPrefix(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := strings.CutPrefix(r.URL.Path, "/api"); ok {
			r.URL.Path = p
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
		}
		next.ServeHTTP(w, r)
	})
}
