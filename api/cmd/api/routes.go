package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/justinas/alice"
	"github.com/xdoubleu/essentia/v4/pkg/middleware"

	"tools.xdoubleu.com/gen/admin/v1/adminv1connect"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/gen/contacts/v1/contactsv1connect"
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

	contactsPath, contactsHandler := contactsv1connect.NewContactsServiceHandler(
		&contactsConnectHandler{app: app},
	)
	mux.Handle(
		"POST "+contactsPath,
		app.services.Auth.Access(contactsHandler.ServeHTTP),
	)

	mux.HandleFunc("GET /api/version", app.versionHandler)

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

	handlers = append(handlers, securityHeadersMiddleware)
	standard := alice.New(append(handlers, app.domainMiddleware)...)
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
