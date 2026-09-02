package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/justinas/alice"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/gen/contacts/v1/contactsv1connect"
	dashboardv1connect "tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/gen/family/v1/familyv1connect"
	"tools.xdoubleu.com/gen/observability/v1/observabilityv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/middleware"
	"tools.xdoubleu.com/internal/oauth2as"
)

//nolint:funlen //route registration: a long list, not complex logic
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

	mux.Handle(
		"POST "+observabilityLogsIngestPath,
		app.observabilityIngestRoute(),
	)

	contactsPath, contactsHandler := contactsv1connect.NewContactsServiceHandler(
		&contactsConnectHandler{app: app},
		scrub,
	)
	mux.Handle(
		"POST "+contactsPath,
		app.auth.Access(contactsHandler.ServeHTTP),
	)

	familyPath, familyHandler := familyv1connect.NewFamilyServiceHandler(
		&familyConnectHandler{app: app},
		scrub,
	)
	mux.Handle(
		"POST "+familyPath,
		app.auth.Access(familyHandler.ServeHTTP),
	)

	dashboardPath, dashboardHandler := dashboardv1connect.NewDashboardServiceHandler(
		&dashboardConnectHandler{app: app},
		scrub,
	)
	mux.Handle(
		"POST "+dashboardPath,
		app.auth.Access(dashboardHandler.ServeHTTP),
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
	// RFC 9728 requires clients discovering metadata for a resource URL with
	// a path (APIURL's "/api" in production) to insert /.well-known/...
	// before that path rather than trust the bare path above — register
	// that alias too, so real client discovery libraries (which compute
	// this URL themselves rather than trusting the bare one) don't 404.
	mux.Handle(
		"/.well-known/oauth-protected-resource/api"+appsMCPPath, appsPRM,
	)

	// Embedded OAuth 2.1 authorization server (issue #1039) backing the MCP
	// flow above — replaces Supabase as the authorization server.
	mux.HandleFunc(
		"GET "+oauth2AuthorizePath,
		oauth2as.AuthorizeHandler(
			app.oauth2as.provider, app.config,
			app.oauth2SessionUserResolver(), app.logger,
		),
	)
	mux.HandleFunc(
		"POST "+oauth2AuthorizePath,
		oauth2as.AuthorizeHandler(
			app.oauth2as.provider, app.config,
			app.oauth2SessionUserResolver(), app.logger,
		),
	)
	mux.HandleFunc(
		"POST "+oauth2TokenPath,
		oauth2as.TokenHandler(app.oauth2as.provider, app.logger),
	)
	mux.HandleFunc(
		"POST "+oauth2RegisterPath,
		oauth2as.RegisterHandler(app.oauth2as.store, app.logger),
	)
	mux.HandleFunc(
		"GET "+oauth2ConsentInfoPath, oauth2as.ConsentInfoHandler(app.oauth2as.store),
	)
	mux.HandleFunc("GET "+oauth2MetadataPath, app.oauth2MetadataHandler())
	// Same RFC 8414 path-insertion alias as above, for AuthIssuer's own path
	// (defaults to APIURL's "/api").
	mux.HandleFunc(
		"GET "+oauth2MetadataPath+"/api", app.oauth2MetadataHandler(),
	)

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
