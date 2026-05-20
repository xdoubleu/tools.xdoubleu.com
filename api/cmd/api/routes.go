package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/justinas/alice"
	"github.com/xdoubleu/essentia/v4/pkg/middleware"

	"tools.xdoubleu.com/cmd/api/internal/logging"
	"tools.xdoubleu.com/gen/admin/v1/adminv1connect"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	bugreportv1connect "tools.xdoubleu.com/gen/bugreport/v1/bugreportv1connect"
	"tools.xdoubleu.com/gen/contacts/v1/contactsv1connect"
	"tools.xdoubleu.com/gen/settings/v1/settingsv1connect"
	"tools.xdoubleu.com/internal/constants"
)

func (app *Application) Routes() http.Handler {
	mux := http.NewServeMux()

	authPath, authHandler := authv1connect.NewAuthServiceHandler(
		&authConnectHandler{app: app},
	)
	mux.Handle("POST "+authPath, authHandler)

	adminPath, adminHandler := adminv1connect.NewAdminServiceHandler(
		&adminConnectHandler{app: app},
	)
	mux.Handle("POST "+adminPath, app.services.Auth.Access(adminHandler.ServeHTTP))

	settingsPath, settingsHandler := settingsv1connect.NewSettingsServiceHandler(
		&settingsConnectHandler{app: app},
	)
	mux.Handle(
		"POST "+settingsPath,
		app.services.Auth.Access(settingsHandler.ServeHTTP),
	)

	contactsPath, contactsHandler := contactsv1connect.NewContactsServiceHandler(
		&contactsConnectHandler{app: app},
	)
	mux.Handle(
		"POST "+contactsPath,
		app.services.Auth.Access(contactsHandler.ServeHTTP),
	)

	bugReportPath, bugReportHandler := bugreportv1connect.NewBugReportServiceHandler(
		&bugReportConnectHandler{app: app},
	)
	mux.Handle(
		"POST "+bugReportPath,
		app.services.Auth.Access(bugReportHandler.ServeHTTP),
	)

	mux.HandleFunc("GET /api/version", app.versionHandler)

	app.imageRoutes("images", mux)

	app.apps.Routes(mux)

	allowedOrigins := []string{app.config.WebURL}
	for _, a := range *app.apps {
		if d := a.GetDomain(); d != "" {
			allowedOrigins = append(allowedOrigins, "https://"+d)
		}
	}

	var (
		handlers []alice.Constructor
		err      error
	)

	if app.config.Throttle {
		handlers, err = middleware.DefaultWithSentry(
			app.logger,
			allowedOrigins,
			app.config.Env,
			"connect-protocol-version",
			"connect-timeout-ms",
		)
		if err != nil {
			panic(err)
		}
	} else {
		handlers = middleware.Minimal(app.logger)
	}

	standard := alice.New(
		append(handlers, app.domainMiddleware, app.requestLogMiddleware)...)
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

			ctx := context.WithValue(
				r.Context(),
				constants.AppDisplayNameContextKey,
				a.GetDisplayName(),
			)
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
