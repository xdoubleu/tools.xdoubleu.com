package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/justinas/alice"
	"github.com/xdoubleu/essentia/v4/pkg/middleware"

	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/gen/contacts/v1/contactsv1connect"
	"tools.xdoubleu.com/gen/observability/v1/observabilityv1connect"
	"tools.xdoubleu.com/gen/profile/v1/profilev1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/constants"
)

func (app *Application) Routes() http.Handler {
	mux := http.NewServeMux()
	scrub := iapp.ScrubInternalErrors(app.logger)

	authPath, authHandler := authv1connect.NewAuthServiceHandler(
		&authConnectHandler{app: app},
		scrub,
	)
	mux.Handle("POST "+authPath, authHandler)

	accessPath, accessHandler := accessv1connect.NewAccessServiceHandler(
		&accessConnectHandler{app: app},
		scrub,
	)
	mux.Handle("POST "+accessPath, app.auth.Access(accessHandler.ServeHTTP))

	obsPath, obsHandler := observabilityv1connect.NewObservabilityServiceHandler(
		&obsConnectHandler{app: app},
		scrub,
	)
	mux.Handle(
		"POST "+obsPath,
		app.auth.Access(obsHandler.ServeHTTP),
	)

	contactsPath, contactsHandler := contactsv1connect.NewContactsServiceHandler(
		&contactsConnectHandler{app: app},
		scrub,
	)
	mux.Handle(
		"POST "+contactsPath,
		app.auth.Access(contactsHandler.ServeHTTP),
	)

	profilePath, profileHandler := profilev1connect.NewProfileServiceHandler(
		&profileConnectHandler{app: app},
		scrub,
	)
	mux.Handle(
		"POST "+profilePath,
		app.auth.Access(profileHandler.ServeHTTP),
	)

	mux.HandleFunc("GET /api/version", app.versionHandler)
	mux.HandleFunc("GET /health", app.healthHandler)

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
	// usageMiddleware runs after domainMiddleware so custom-domain requests
	// are already rewritten to /<app>/… before their labels are derived.
	handlers = append(handlers, app.domainMiddleware, app.usageMiddleware)
	standard := alice.New(handlers...)
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
