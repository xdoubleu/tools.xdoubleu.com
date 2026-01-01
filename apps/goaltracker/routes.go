package goaltracker

import (
	"fmt"
	"net/http"

	"github.com/XDoubleU/essentia/pkg/middleware"
	"github.com/getsentry/sentry-go"
	"github.com/justinas/alice"
)

func (app *GoalTracker) apiRoutes(prefix string, mux *http.ServeMux) {
	apiPrefix := fmt.Sprintf("/%s/api", prefix)
	app.goalsRoutes(apiPrefix, mux)
	app.progressRoutes(apiPrefix, mux)
}

func (app *GoalTracker) Routes(prefix string, mux *http.ServeMux) http.Handler {
	app.templateRoutes(prefix, mux)
	app.apiRoutes(prefix, mux)

	var sentryClientOptions sentry.ClientOptions
	if len(app.Config.SentryDsn) > 0 {
		//nolint:exhaustruct //other fields are optional
		sentryClientOptions = sentry.ClientOptions{
			Dsn:              app.Config.SentryDsn,
			Environment:      app.Config.Env,
			Release:          app.Config.Release,
			EnableTracing:    true,
			TracesSampleRate: app.Config.SampleRate,
			SampleRate:       app.Config.SampleRate,
		}
	}

	allowedOrigins := []string{app.Config.WebURL}
	handlers, err := middleware.DefaultWithSentry(
		app.logger,
		allowedOrigins,
		app.Config.Env,
		sentryClientOptions,
	)

	if err != nil {
		panic(err)
	}

	standard := alice.New(handlers...)
	return standard.Then(mux)
}
