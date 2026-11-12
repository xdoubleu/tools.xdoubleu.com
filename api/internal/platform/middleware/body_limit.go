package middleware

import "net/http"

// MaxBodyBytes caps every request body at n bytes; a longer body fails its
// read. Large files go straight to R2 via presigned URLs, never through here.
func MaxBodyBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, n)
	}
}
