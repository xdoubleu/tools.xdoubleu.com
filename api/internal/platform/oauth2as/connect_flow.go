package oauth2as

import (
	"log/slog"
	"net/http"

	"golang.org/x/oauth2"

	"tools.xdoubleu.com/internal/config"
	"tools.xdoubleu.com/internal/kit/constants"
	"tools.xdoubleu.com/internal/kit/contexttools"
	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/oauthconn"
	"tools.xdoubleu.com/internal/platform/github"
	"tools.xdoubleu.com/internal/platform/sentryapi"
	"tools.xdoubleu.com/internal/repositories"
)

// Browser-facing OAuth connect flow for GitHub/Sentry. Plain HTTP, not
// Connect: the start leg 302s to the provider and the callback is the
// provider's redirect. Both legs are AdminAccess-gated; CSRF state binds the
// admin's user ID so connected_by survives the external redirect.

// ConnectProvider is one provider the connect flow can authorize against.
type ConnectProvider struct {
	Provider models.OAuthProvider
	Conf     func(cfg config.Config) *oauth2.Config
}

// AdminGate wraps a handler so only admins reach it.
type AdminGate interface {
	AdminAccess(next http.HandlerFunc) http.HandlerFunc
}

// ConnectFlow serves the start and callback legs of the provider connect flow.
type ConnectFlow struct {
	Providers map[string]ConnectProvider
	Config    config.Config
	Admin     AdminGate
	State     *oauthconn.StateStore
	Conns     *repositories.OAuthConnectionsRepository
	Logger    *slog.Logger
}

// NewConnectFlow wires the GitHub and Sentry providers.
func NewConnectFlow(
	cfg config.Config,
	admin AdminGate,
	state *oauthconn.StateStore,
	conns *repositories.OAuthConnectionsRepository,
	logger *slog.Logger,
) *ConnectFlow {
	return &ConnectFlow{
		Providers: map[string]ConnectProvider{
			"github": {
				Provider: models.OAuthProviderGithub,
				Conf: func(c config.Config) *oauth2.Config {
					return github.OAuthConfig(
						c.GithubOAuthClientID,
						c.GithubOAuthClientSecret,
						c.APIURL,
					)
				},
			},
			"sentry": {
				Provider: models.OAuthProviderSentry,
				Conf: func(c config.Config) *oauth2.Config {
					return sentryapi.OAuthConfig(
						c.SentryOAuthClientID,
						c.SentryOAuthClientSecret,
						c.APIURL,
					)
				},
			},
		},
		Config: cfg,
		Admin:  admin,
		State:  state,
		Conns:  conns,
		Logger: logger,
	}
}

// StartRoute redirects an admin to the provider's consent page.
func (f *ConnectFlow) StartRoute() http.HandlerFunc {
	return f.Admin.AdminAccess(func(w http.ResponseWriter, r *http.Request) {
		def, ok := f.Providers[r.PathValue("provider")]
		if !ok {
			http.NotFound(w, r)
			return
		}

		user := contexttools.GetValue[models.User](
			r.Context(),
			constants.UserContextKey,
		)
		state := f.State.New(def.Provider, user.ID)
		authURL := def.Conf(f.Config).AuthCodeURL(state, oauth2.AccessTypeOffline)
		http.Redirect(w, r, authURL, http.StatusFound)
	})
}

// CallbackRoute exchanges the provider's code and stores the connection.
func (f *ConnectFlow) CallbackRoute() http.HandlerFunc {
	return f.Admin.AdminAccess(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("provider")
		def, ok := f.Providers[name]
		if !ok {
			http.NotFound(w, r)
			return
		}

		provider, userID, ok := f.State.Consume(r.URL.Query().Get("state"))
		if !ok || provider != def.Provider {
			http.Error(w, "invalid or expired oauth state", http.StatusBadRequest)
			return
		}

		conf := def.Conf(f.Config)
		code := r.URL.Query().Get("code")
		tok, err := conf.Exchange(r.Context(), code)
		if err != nil {
			f.Logger.ErrorContext(r.Context(), "oauth exchange failed",
				slog.String("provider", name), slog.Any("error", err))
			http.Redirect(
				w, r, f.Config.WebURL+"/monitoring/connections?oauth_error="+name,
				http.StatusFound,
			)
			return
		}

		if storeErr := f.Conns.Upsert(
			r.Context(), def.Provider, tok, userID, conf.Scopes,
		); storeErr != nil {
			f.Logger.ErrorContext(r.Context(), "failed to store oauth connection",
				slog.String("provider", name), slog.Any("error", storeErr))
			http.Error(w, "failed to store connection", http.StatusInternalServerError)
			return
		}

		http.Redirect(
			w, r, f.Config.WebURL+"/monitoring/connections?oauth_connected="+name,
			http.StatusFound,
		)
	})
}
