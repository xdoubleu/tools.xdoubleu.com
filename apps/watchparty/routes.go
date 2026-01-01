package watchparty

import (
	"fmt"
	"net/http"

	"github.com/XDoubleU/essentia/pkg/middleware"
	"github.com/getsentry/sentry-go"
	"github.com/justinas/alice"
)

func (app *WatchParty) apiRoutes(prefix string, mux *http.ServeMux) {
	apiPrefix := fmt.Sprintf("/%s/api", prefix)

	app.wsRoutes(apiPrefix, mux)
}

func (app *WatchParty) Routes(prefix string, mux *http.ServeMux) http.Handler {
	app.templateRoutes(prefix, mux)
	app.apiRoutes(prefix, mux)

	var sentryClientOptions sentry.ClientOptions
	if len(app.config.SentryDsn) > 0 {
		//nolint:exhaustruct //other fields are optional
		sentryClientOptions = sentry.ClientOptions{
			Dsn:              app.config.SentryDsn,
			Environment:      app.config.Env,
			Release:          app.config.Release,
			EnableTracing:    true,
			TracesSampleRate: app.config.SampleRate,
			SampleRate:       app.config.SampleRate,
		}
	}

	allowedOrigins := []string{app.config.WebURL}
	handlers, err := middleware.DefaultWithSentry(
		app.logger,
		allowedOrigins,
		app.config.Env,
		sentryClientOptions,
	)

	if err != nil {
		panic(err)
	}

	standard := alice.New(handlers...)
	return standard.Then(mux)
}
