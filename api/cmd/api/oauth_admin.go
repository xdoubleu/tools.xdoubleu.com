package main

import (
	"log/slog"
	"net/http"

	"github.com/xdoubleu/essentia/v4/pkg/contexttools"
	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/constants"
	"tools.xdoubleu.com/internal/digitalocean"
	"tools.xdoubleu.com/internal/github"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/sentryapi"
)

// The browser-facing legs of the OAuth connect flow for the observability
// integrations (GitHub/Sentry/DigitalOcean, issue #440). These are plain HTTP
// routes, not ConnectRPC: the start leg must issue a 302 redirect to the
// provider's authorize URL, and the callback leg is invoked directly by the
// provider's own browser redirect (?code=&state=) — neither fits Connect's
// POST-JSON/protobuf contract, mirroring how the existing MCP OAuth
// protected-resource metadata is also plain mux.Handle (cmd/api/mcp.go).
//
// Both legs are gated by the existing cookie-session AdminAccess middleware.
// CSRF state additionally binds the resolved admin's user ID, so
// connected_by doesn't depend on the cookie surviving the external redirect.

type oauthProviderDef struct {
	provider models.OAuthProvider
	conf     func(app *Application) *oauth2.Config
}

//nolint:gochecknoglobals // fixed provider table, not runtime-configurable
var oauthProviders = map[string]oauthProviderDef{
	"github": {
		provider: models.OAuthProviderGithub,
		conf: func(app *Application) *oauth2.Config {
			return github.OAuthConfig(
				app.config.GithubOAuthClientID,
				app.config.GithubOAuthClientSecret,
				app.config.APIURL,
			)
		},
	},
	"sentry": {
		provider: models.OAuthProviderSentry,
		conf: func(app *Application) *oauth2.Config {
			return sentryapi.OAuthConfig(
				app.config.SentryOAuthClientID,
				app.config.SentryOAuthClientSecret,
				app.config.APIURL,
			)
		},
	},
	"digitalocean": {
		provider: models.OAuthProviderDigitalOcean,
		conf: func(app *Application) *oauth2.Config {
			return digitalocean.OAuthConfig(
				app.config.DOOAuthClientID,
				app.config.DOOAuthClientSecret,
				app.config.APIURL,
			)
		},
	},
}

func (app *Application) oauthStartRoute() http.HandlerFunc {
	return app.auth.AdminAccess(func(w http.ResponseWriter, r *http.Request) {
		def, ok := oauthProviders[r.PathValue("provider")]
		if !ok {
			http.NotFound(w, r)
			return
		}

		user := contexttools.GetValue[models.User](
			r.Context(),
			constants.UserContextKey,
		)
		state := app.oauthState.New(def.provider, user.ID)
		authURL := def.conf(app).AuthCodeURL(state, oauth2.AccessTypeOffline)
		http.Redirect(w, r, authURL, http.StatusFound)
	})
}

func (app *Application) oauthCallbackRoute() http.HandlerFunc {
	return app.auth.AdminAccess(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("provider")
		def, ok := oauthProviders[name]
		if !ok {
			http.NotFound(w, r)
			return
		}

		provider, userID, ok := app.oauthState.Consume(r.URL.Query().Get("state"))
		if !ok || provider != def.provider {
			http.Error(w, "invalid or expired oauth state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		tok, err := def.conf(app).Exchange(r.Context(), code)
		if err != nil {
			app.logger.ErrorContext(r.Context(), "oauth exchange failed",
				slog.String("provider", name), slog.Any("error", err))
			http.Redirect(
				w, r, app.config.WebURL+"/monitoring?oauth_error="+name,
				http.StatusFound,
			)
			return
		}

		if storeErr := app.oauthConnRepo.Upsert(
			r.Context(), def.provider, tok, userID,
		); storeErr != nil {
			app.logger.ErrorContext(r.Context(), "failed to store oauth connection",
				slog.String("provider", name), slog.Any("error", storeErr))
			http.Error(w, "failed to store connection", http.StatusInternalServerError)
			return
		}

		http.Redirect(
			w, r, app.config.WebURL+"/monitoring?oauth_connected="+name,
			http.StatusFound,
		)
	})
}
