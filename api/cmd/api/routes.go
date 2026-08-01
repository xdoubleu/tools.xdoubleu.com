package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/justinas/alice"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/xdoubleu/essentia/v4/pkg/middleware"

	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/gen/contacts/v1/contactsv1connect"
	"tools.xdoubleu.com/gen/observability/v1/observabilityv1connect"
	"tools.xdoubleu.com/gen/profile/v1/profilev1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/constants"
)

// deployLogsWriteDeadline bounds how long the GetDeployLogs response is
// allowed to take. It fetches BUILD/DEPLOY/RUN/RUN_RESTARTED logs per
// service component from DigitalOcean and exceeds the server's global 10s
// httpWriteTimeout (main.go): that timeout doesn't cancel the handler, it
// just makes the eventual response write silently fail once the deadline has
// passed. This route gets its own longer write deadline and a matching
// context timeout so a genuinely stuck upstream call cancels the handler and
// fails loudly instead of hanging past it.
//
// Kept under DigitalOcean App Platform's own ~25s edge request timeout
// (issue #672): a value above it let the edge reset the upstream connection
// (503 UC) before this deadline fired, which is a silent failure — no log
// line, no Sentry event. Firing first turns that into a logged, Sentry-
// reported error. The healthy path completes in well under this (live reads
// are bounded by liveLogDeadline), so it only bites a genuinely stuck call.
const deployLogsWriteDeadline = 20 * time.Second

// withExtendedDeadline raises the response write deadline (net/http's
// ResponseController) and the request context's deadline together for one
// slow route, without touching the server-wide httpWriteTimeout that every
// other (fast) route relies on.
func withExtendedDeadline(d time.Duration, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).SetWriteDeadline(
			time.Now().Add(d),
		); err != nil {
			// Not all ResponseWriters support deadlines (e.g. in tests using
			// httptest.ResponseRecorder) — fall back to the ambient deadline.
			next.ServeHTTP(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

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
	// More specific than the prefix registration above, so net/http's mux
	// routes this one procedure through the extended deadline instead.
	mux.Handle(
		"POST "+observabilityv1connect.ObservabilityServiceGetDeployLogsProcedure,
		app.auth.Access(
			withExtendedDeadline(deployLogsWriteDeadline, obsHandler.ServeHTTP),
		),
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

	// MCP server (read-only) behind OAuth 2.1 Bearer auth — every app's own
	// read RPCs plus the admin observability tools. The protected-resource
	// metadata is public (unauthenticated) for client discovery; the MCP
	// endpoint itself verifies the Bearer token.
	appsPRM := mcpauth.ProtectedResourceMetadataHandler(
		app.mcpResourceMetadataFor(appsMCPPath, "tools.xdoubleu.com apps"),
	)
	mux.Handle(appsResourceMetadataPath, appsPRM)
	mux.Handle(rootResourceMetadataPath, appsPRM)
	mux.Handle(appsMCPPath, app.appsMCPRoute())

	// Browser-facing OAuth connect flow for the observability integrations
	// (issue #440) — plain HTTP because the provider redirect can't carry
	// Connect's protocol framing. Both legs are admin-cookie-gated.
	mux.HandleFunc("GET /admin/oauth/{provider}/start", app.oauthStartRoute())
	mux.HandleFunc("GET /admin/oauth/{provider}/callback", app.oauthCallbackRoute())

	mux.HandleFunc("GET /api/version", app.versionHandler)
	mux.HandleFunc("GET "+healthPath, app.healthHandler)

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
