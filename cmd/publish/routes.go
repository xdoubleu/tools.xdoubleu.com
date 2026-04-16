package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/justinas/alice"
	"github.com/xdoubleu/essentia/v3/pkg/middleware"
	"github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
)

func (app *Application) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", app.services.Auth.TemplateAccess(app.Home))
	mux.HandleFunc(
		"POST /api/bug-report",
		app.services.Auth.Access(app.bugReportHandler),
	)

	app.authRoutes("auth", mux)
	app.imageRoutes("images", mux)

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

	standard := alice.New(append(handlers, app.requestLogMiddleware)...)
	return standard.Then(mux)
}

// statusWriter captures the HTTP status code written by a handler.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (app *Application) requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//nolint:exhaustruct // ID is intentionally empty until auth fills it
		carrier := &logging.UserIDCarrier{}
		ctx := context.WithValue(r.Context(), logging.CarrierKey, carrier)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(ctx))
		if carrier.ID != "" {
			app.requestBuffer.Record(carrier.ID, logging.LogEntry{
				Time:    time.Now(),
				Level:   "REQUEST",
				Message: fmt.Sprintf("%s %s → %d", r.Method, r.URL.Path, sw.status),
			})
		}
	})
}

func (app *Application) Home(w http.ResponseWriter, _ *http.Request) {
	data := []string{}
	for _, a := range *app.apps {
		data = append(data, a.GetName())
	}

	tpl.RenderWithPanic(app.tpl, w, "home.html", data)
}
