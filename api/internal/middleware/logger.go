package middleware

import (
	"log/slog"
	"net/http"
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
			slog.String("endpoint", r.RequestURI),
			slog.Duration("duration", time.Since(t)),
		)
	})
}
