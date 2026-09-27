package middleware

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/contexttools"
)

// Logger adds a logger to the context and logs each request with its duration.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return loggerHandler(logger, next)
	}
}

func loggerHandler(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := httptools.NewResponseWriter(w)
		t := time.Now()

		r = r.WithContext(contexttools.WithLogger(r.Context(), logger))

		next.ServeHTTP(rw, r)

		logger.Info(
			"processed request",
			slog.Int("status", rw.Status()),
			slog.String("endpoint", RedactPath(r.RequestURI)),
			slog.Duration("duration", time.Since(t)),
		)
	})
}

// koboTokenSegment matches the Kobo sync token, a bearer credential carried
// as a path segment.
var koboTokenSegment = regexp.MustCompile(`(/kobo/)[^/?#]+`) //nolint:gochecknoglobals //compiled once

// RedactPath masks credentials in a request path before it is logged or sent
// to Sentry: the Kobo device token, and the query string.
func RedactPath(uri string) string {
	path, _, _ := strings.Cut(uri, "?")
	return koboTokenSegment.ReplaceAllString(path, "${1}redacted")
}
