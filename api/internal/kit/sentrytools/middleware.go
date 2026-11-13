package sentrytools

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

// Middleware wires the request-scoped Sentry hub middleware; call [Init]
// first. Tests point the global hub at a mocked transport via
// [MockedSentryClientOptions] before building the chain.
func Middleware() (func(http.Handler) http.Handler, error) {
	//nolint:exhaustruct //other fields are optional
	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic: true,
	})
	return func(next http.Handler) http.Handler {
		return sentryHandler.Handle(next)
	}, nil
}
