package main

import (
	"context"
	"net/http"
	"strings"

	"connectrpc.com/connect/v2"
	"github.com/justinas/alice"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"

	"tools.xdoubleu.com/gen/access/v1/accessv1connect"
	"tools.xdoubleu.com/gen/auth/v1/authv1connect"
	"tools.xdoubleu.com/gen/books/v1/booksv1connect"
	dashboardv1connect "tools.xdoubleu.com/gen/dashboard/v1/dashboardv1connect"
	"tools.xdoubleu.com/gen/family/v1/familyv1connect"
	iapp "tools.xdoubleu.com/internal/app"
	"tools.xdoubleu.com/internal/communication/httptools"
	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/middleware"
	"tools.xdoubleu.com/internal/oauth2as"
	"tools.xdoubleu.com/internal/obs"
)

// maxRequestBodyBytes bounds every request body, e.g. inbound newsletter
// emails and CSV imports; book files upload straight to R2.
const maxRequestBodyBytes = 10 << 20

func (app *Application) Routes() http.Handler {
	return app.middleware().Then(app.mux())
}

// mux registers every route, without the middleware chain.
func (app *Application) mux() *http.ServeMux {
	mux := http.NewServeMux()
	mountConnect := func(gate iapp.Gate, register func(*connect.Server)) {
		iapp.MountConnect(mux, app.logger, gate, register)
	}

	mountConnect(nil, func(s *connect.Server) {
		authv1connect.RegisterAuthServiceHandler(s, &authConnectHandler{app: app})
	})
	mountConnect(app.auth.Access, func(s *connect.Server) {
		accessv1connect.RegisterAccessServiceHandler(s, &accessConnectHandler{app: app})
		familyv1connect.RegisterFamilyServiceHandler(
			s,
			&familyConnectHandler{app: app},
		)
		dashboardv1connect.RegisterDashboardServiceHandler(
			s,
			&dashboardConnectHandler{app: app},
		)
	})
	obs.Register(mux, app.logger, app.auth.Access, app.obsDeps())

	// MCP behind OAuth 2.1 Bearer auth. The protected-resource metadata is public
	// for discovery.
	appsPRM := mcpauth.ProtectedResourceMetadataHandler(
		app.mcpResourceMetadataFor(appsMCPPath, "tools.xdoubleu.com apps"),
	)
	mux.Handle(appsResourceMetadataPath, appsPRM)
	mux.Handle(rootResourceMetadataPath, appsPRM)
	mux.Handle(appsMCPPath, app.appsMCPRoute())
	// RFC 9728 path-insertion alias: clients insert /.well-known/... before the
	// resource path ("/api"), so register that URL too.
	mux.Handle(
		"/.well-known/oauth-protected-resource/api"+appsMCPPath, appsPRM,
	)

	// Embedded OAuth 2.1 authorization server.
	mux.HandleFunc(
		"GET "+oauth2AuthorizePath,
		oauth2as.AuthorizeHandler(
			app.oauth2as.provider, app.config,
			app.oauth2SessionUserResolver(), app.oauth2as.oidcKeyID(), app.logger,
		),
	)
	mux.HandleFunc(
		"POST "+oauth2AuthorizePath,
		oauth2as.AuthorizeHandler(
			app.oauth2as.provider, app.config,
			app.oauth2SessionUserResolver(), app.oauth2as.oidcKeyID(), app.logger,
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
		"GET "+oauth2ConsentInfoPath, oauth2as.ConsentInfoHandler(
			app.oauth2as.store, app.config, app.oauth2SessionUserResolver(),
		),
	)
	mux.HandleFunc("GET "+oauth2MetadataPath, app.oauth2MetadataHandler())
	// Same path-insertion alias for AuthIssuer's path.
	mux.HandleFunc(
		"GET "+oauth2MetadataPath+"/api", app.oauth2MetadataHandler(),
	)
	// OIDC discovery and the JWKS Grafana uses to verify ID tokens.
	mux.HandleFunc("GET "+openIDConfigurationPath, app.oauth2MetadataHandler())
	mux.HandleFunc("GET "+openIDConfigurationPath+"/api", app.oauth2MetadataHandler())
	mux.HandleFunc(
		"GET "+oauth2JWKSPath, oauth2as.JWKSHandler(app.oauth2as.oidcKey),
	)

	// OAuth connect flow for observability integrations; plain HTTP,
	// admin-cookie-gated.
	mux.HandleFunc("GET /admin/oauth/{provider}/start", app.oauthStartRoute())
	mux.HandleFunc("GET /admin/oauth/{provider}/callback", app.oauthCallbackRoute())

	mux.HandleFunc("GET /api/version", app.versionHandler)
	mux.HandleFunc("GET "+healthPath, app.healthHandler)
	mux.Handle("GET "+metricsPath, metricsHandler(app.config.ObservabilityIngestSecret))

	app.apps.Routes(mux)

	return mux
}

// middleware is the chain in front of every route.
func (app *Application) middleware() alice.Chain {
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

	handlers = append(
		handlers, securityHeadersMiddleware, middleware.MaxBodyBytes(maxRequestBodyBytes),
	)
	// usageMiddleware runs after domainMiddleware so custom-domain requests
	// are already rewritten to /<app>/… before their labels are derived.
	handlers = append(handlers, app.domainMiddleware, app.usageMiddleware)
	// Budget copies the request (WithContext), so RequestDuration must come
	// after it and hand the mux that same copy: the mux sets Pattern on it, and
	// RequestDuration labels with r.Pattern for Grafana's RequestP95High alert.
	// Nothing may copy the request between RequestDuration and the mux.
	handlers = append(handlers, httptools.Budget(app.logger, isLongRequest))
	handlers = append(handlers, middleware.RequestDuration())
	return alice.New(handlers...)
}

// isLongRequest marks the routes that legitimately outlast
// httptools.RequestBudget: FinalizeBookUpload streams up to 250 MB from R2,
// and MCP tool calls wait on external APIs.
func isLongRequest(r *http.Request) bool {
	return r.URL.Path == booksv1connect.BookFilesServiceFinalizeBookUploadProcedure ||
		r.URL.Path == appsMCPPath
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
