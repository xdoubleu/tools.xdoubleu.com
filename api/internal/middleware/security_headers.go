package middleware

import "net/http"

// SecurityHeaders sets the baseline security response headers and drops
// X-Powered-By.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-DNS-Prefetch-Control", "off")
		h.Set("X-Download-Options", "noopen")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Del("X-Powered-By")
		h.Set("Strict-Transport-Security", "max-age=5184000; includeSubDomains")
		h.Set("X-XSS-Protection", "1; mode=block")

		next.ServeHTTP(w, r)
	})
}
