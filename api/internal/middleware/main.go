// Package middleware provides middleware and predefined chains.
package middleware

import (
	"log/slog"
	"time"

	"github.com/goddtriffin/helmet"
	"github.com/justinas/alice"

	"tools.xdoubleu.com/internal/sentrytools"
)

// Minimal is [Logger] and [Recover].
func Minimal(logger *slog.Logger) []alice.Constructor {
	return []alice.Constructor{
		Logger(logger),
		Recover(logger),
	}
}

// Default is [Minimal] plus helmet, [CORS] and [RateLimit].
func Default(
	logger *slog.Logger,
	allowedOrigins []string,
	extraHeaders ...string,
) ([]alice.Constructor, error) {
	return defaultBase(logger, allowedOrigins, nil, extraHeaders...)
}

// DefaultWithSentry is [Default] plus [sentrytools.Middleware]. Call
// [tools.xdoubleu.com/sentrytools.Init] first.
func DefaultWithSentry(
	logger *slog.Logger,
	allowedOrigins []string,
	env string,
	extraHeaders ...string,
) ([]alice.Constructor, error) {
	return defaultBase(logger, allowedOrigins, &env, extraHeaders...)
}

func defaultBase(
	logger *slog.Logger,
	allowedOrigins []string,
	env *string,
	extraHeaders ...string,
) ([]alice.Constructor, error) {
	useSentry := env != nil

	helmet := helmet.Default()

	handlers := Minimal(logger)
	handlers = append(handlers, helmet.Secure)
	handlers = append(handlers, CORS(allowedOrigins, useSentry, extraHeaders...))
	//nolint:mnd//no magic number
	handlers = append(handlers, RateLimit(10, 30, time.Minute, 3*time.Minute))

	if useSentry {
		sentryMiddleware, err := sentrytools.Middleware(*env)
		if err != nil {
			return nil, err
		}

		handlers = append(handlers, sentryMiddleware)
	}

	return handlers, nil
}
