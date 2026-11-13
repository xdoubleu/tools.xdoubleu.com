package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

// CORS is middleware used to apply CORS settings.
func CORS(
	allowedOrigins []string,
	useSentry bool,
	extraHeaders ...string,
) func(http.Handler) http.Handler {
	allowedHeaders := []string{"content-type"}
	if useSentry {
		allowedHeaders = append(allowedHeaders, "baggage", "sentry-trace")
	}
	allowedHeaders = append(allowedHeaders, extraHeaders...)

	//nolint:exhaustruct //other fields are optional
	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE"},
		AllowedHeaders:   allowedHeaders,
	})

	return c.Handler
}
