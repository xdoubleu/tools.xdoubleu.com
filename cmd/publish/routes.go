package main

import (
	"net/http"

	"github.com/XDoubleU/essentia/pkg/middleware"
	"github.com/XDoubleU/essentia/pkg/tpl"
	"github.com/getsentry/sentry-go"
	"github.com/justinas/alice"
)

func (app *Application) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", app.services.Auth.TemplateAccess(app.Home))

	mux.HandleFunc("GET /proxy", app.Proxy)

	app.authRoutes("api", mux)

	app.apps.Routes(mux)

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

func (app *Application) Home(w http.ResponseWriter, _ *http.Request) {
	data := []string{}
	for _, a := range app.apps.apps {
		data = append(data, a.GetName())
	}

	tpl.RenderWithPanic(app.tpl, w, "home.html", data)
}
