package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover is middleware used to recover from a panic.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return recoverHandler(logger, next)
	}
}

func recoverHandler(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("connection", "close")
				w.WriteHeader(http.StatusInternalServerError)
				logger.ErrorContext(
					r.Context(),
					"PANIC",
					slog.Any("error", err),
					slog.String("stacktrace", string(debug.Stack())),
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
