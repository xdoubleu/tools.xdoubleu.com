package sentrytools

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"

	"tools.xdoubleu.com/internal/config"
)

// Middleware is middleware used to configure and enable Sentry.
// Call [tools.xdoubleu.com/sentrytools.Init] at application startup before
// using this middleware.
// When env is [config.TestEnv], a mocked [sentry.Hub] will be used and
// Sentry is self-initialized with mock options.
func Middleware(env string) (func(http.Handler) http.Handler, error) {
	isTestEnv := env == config.TestEnv

	if isTestEnv {
		if err := sentry.Init(MockedSentryClientOptions()); err != nil {
			return nil, err
		}
	}

	//nolint:exhaustruct //other fields are optional
	sentryHandler := sentryhttp.New(sentryhttp.Options{
		Repanic: true,
	})

	if isTestEnv {
		return func(next http.Handler) http.Handler {
			return sentryHandler.Handle(useMockedHub(next))
		}, nil
	}

	return func(next http.Handler) http.Handler {
		return sentryHandler.Handle(next)
	}, nil
}

func useMockedHub(next http.Handler) http.Handler {
	mockedHub := MockedSentryHub()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentry.SetHubOnContext(r.Context(), mockedHub)
		next.ServeHTTP(w, r)
	})
}
