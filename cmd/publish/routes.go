package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/justinas/alice"
	"github.com/xdoubleu/essentia/v3/pkg/middleware"
	"github.com/xdoubleu/essentia/v3/pkg/tpl"
	"tools.xdoubleu.com/cmd/publish/internal/logging"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/models"
)

func (app *Application) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", app.services.Auth.TemplateAccess(app.Home))
	mux.HandleFunc("GET /admin", app.services.Auth.AdminAccess(app.adminHandler))
	mux.HandleFunc(
		"POST /admin/users/{userID}/role",
		app.services.Auth.AdminAccess(app.adminSetRoleHandler),
	)
	mux.HandleFunc(
		"POST /admin/users/{userID}/access/{appName}",
		app.services.Auth.AdminAccess(app.adminSetAppAccessHandler),
	)
	mux.HandleFunc(
		"GET /settings",
		app.services.Auth.TemplateAccess(app.settingsHandler),
	)
	mux.HandleFunc(
		"POST /settings",
		app.services.Auth.TemplateAccess(app.saveSettingsHandler),
	)
	mux.HandleFunc(
		"GET /onboarding",
		app.services.Auth.TemplateAccess(app.onboardingHandler),
	)
	mux.HandleFunc(
		"POST /onboarding",
		app.services.Auth.TemplateAccess(app.saveOnboardingHandler),
	)
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
	for _, a := range *app.apps {
		if d := a.GetDomain(); d != "" {
			allowedOrigins = append(allowedOrigins, "https://"+d)
		}
	}

	handlers, err := middleware.DefaultWithSentry(
		app.logger,
		allowedOrigins,
		app.config.Env,
		sentryClientOptions,
	)

	if err != nil {
		panic(err)
	}

	standard := alice.New(append(handlers, app.domainMiddleware, app.requestLogMiddleware)...)
	return standard.Then(mux)
}

func (app *Application) domainMiddleware(next http.Handler) http.Handler {
	domainToApp := make(map[string]App)
	for _, a := range *app.apps {
		if d := a.GetDomain(); d != "" {
			domainToApp[d] = a
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if i := strings.IndexByte(host, ':'); i != -1 {
			host = host[:i]
		}

		if a, ok := domainToApp[host]; ok {
			originalPath := r.URL.Path
			prefix := "/" + a.GetName()
			if r.URL.Path == "/" {
				r.URL.Path = prefix + "/"
			} else {
				r.URL.Path = prefix + r.URL.Path
			}

			ctx := context.WithValue(r.Context(), constants.AppDisplayNameContextKey, a.GetDisplayName())
			ctx = context.WithValue(ctx, constants.OriginalPathContextKey, originalPath)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
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

func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := sw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf(
			"underlying ResponseWriter does not implement http.Hijacker",
		)
	}
	return hj.Hijack()
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

func (app *Application) Home(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	data := []string{}
	for _, a := range *app.apps {
		if user != nil && (user.Role == models.RoleAdmin ||
			slices.Contains(user.AppAccess, a.GetName())) {
			data = append(data, a.GetName())
		}
	}

	tpl.RenderWithPanic(app.tpl, w, "home.html", data)
}
