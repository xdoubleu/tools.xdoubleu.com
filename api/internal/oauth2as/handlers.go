package oauth2as

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/openid"

	"tools.xdoubleu.com/internal/config"
)

// SessionUserResolver resolves the web session's user from its cookie. It is
// injected by cmd/api to avoid an auth <-> oauth2as import cycle.
type SessionUserResolver func(r *http.Request) (user ResolvedUser, ok bool)

// AuthorizeHandler serves /oauth2/authorize: first hit redirects to the web
// consent page; consent=allow re-verifies the session and completes,
// consent=deny returns access_denied.
func AuthorizeHandler(
	provider fosite.OAuth2Provider,
	cfg config.Config,
	resolveUser SessionUserResolver,
	signingKeyID string,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		consent := r.URL.Query().Get("consent")

		if consent == "" {
			//nolint:gosec // base URL is the server's own cfg.WebURL, not
			// user input — only the query string (the OAuth request
			// parameters the consent page needs) is taken from the request.
			http.Redirect(
				w, r, cfg.WebURL+"/oauth/consent?"+r.URL.RawQuery, http.StatusFound,
			)
			return
		}

		ar, err := provider.NewAuthorizeRequest(ctx, r)
		if err != nil {
			logOAuthError(ctx, logger, endpointAuthorize, ar, err)
			provider.WriteAuthorizeError(ctx, w, ar, err)
			return
		}

		if consent == "deny" {
			// Declining is normal; logged only for visibility.
			logOAuthError(
				ctx, logger, endpointAuthorize, ar, fosite.ErrAccessDenied,
			)
			provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrAccessDenied)
			return
		}

		user, ok := resolveUser(r)
		if !ok {
			logOAuthError(
				ctx, logger, endpointAuthorize, ar, fosite.ErrRequestUnauthorized,
			)
			provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrRequestUnauthorized)
			return
		}

		// Grant the requested scopes: fosite only issues a refresh token when
		// offline_access is granted, not merely requested.
		for _, scope := range ar.GetRequestedScopes() {
			ar.GrantScope(scope)
		}
		grantOfflineAccess(ar)

		session := newAuthorizeSession(user, signingKeyID, ar.GetGrantedScopes())
		resp, err := provider.NewAuthorizeResponse(ctx, ar, session)
		if err != nil {
			logOAuthError(ctx, logger, endpointAuthorize, ar, err)
			provider.WriteAuthorizeError(ctx, w, ar, err)
			return
		}

		provider.WriteAuthorizeResponse(ctx, w, ar, resp)
	}
}

// TokenHandler serves /oauth2/token (authorization_code, refresh_token, and
// client_credentials for machine clients).
func TokenHandler(
	provider fosite.OAuth2Provider,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// The OIDC handlers need an openid session; for the MCP flow it behaves like
		// the default one.
		session := openid.NewDefaultSession()
		ar, err := provider.NewAccessRequest(ctx, r, session)
		if err != nil {
			logOAuthError(ctx, logger, endpointToken, ar, err)
			provider.WriteAccessError(ctx, w, ar, err)
			return
		}

		if ar.GetGrantTypes().ExactOne("client_credentials") {
			subject, ok := machineSubject(ar.GetClient().GetID())
			if !ok {
				err = fosite.ErrUnauthorizedClient.WithHint(
					"The client has no machine identity.",
				)
				logOAuthError(ctx, logger, endpointToken, ar, err)
				provider.WriteAccessError(ctx, w, ar, err)
				return
			}
			session.Subject = subject
		}

		resp, err := provider.NewAccessResponse(ctx, ar)
		if err != nil {
			logOAuthError(ctx, logger, endpointToken, ar, err)
			provider.WriteAccessError(ctx, w, ar, err)
			return
		}

		provider.WriteAccessResponse(ctx, w, ar, resp)
	}
}

type registerRequest struct {
	RedirectURIs []string `json:"redirect_uris"`
	ClientName   string   `json:"client_name"`
}

type registerResponse struct {
	ClientID                string   `json:"client_id"`
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	// Scope echoes the assigned scope (RFC 7591 §3.2.1) so clients ask for
	// offline_access.
	Scope string `json:"scope"`
}

// RegisterHandler serves RFC 7591 registration at /oauth2/register.
func RegisterHandler(store *Store, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logRegisterError(
				ctx, logger, errors.New("malformed registration body"),
			)
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		client, err := RegisterClient(ctx, store.db, ClientMetadata(req))
		if err != nil {
			logRegisterError(ctx, logger, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(registerResponse{
			ClientID:                client.ID,
			RedirectURIs:            client.RedirectURIs,
			ClientName:              req.ClientName,
			TokenEndpointAuthMethod: "none",
			GrantTypes:              client.GrantTypes,
			ResponseTypes:           client.ResponseTypes,
			Scope:                   strings.Join(client.Scopes, " "),
		})
	}
}

// ConsentInfoHandler serves GET /oauth2/consent-info: the pending request's
// client name and the scope approval will grant.
func ConsentInfoHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		name, err := store.GetClientName(r.Context(), clientID)
		if err != nil {
			http.Error(w, "unknown client", http.StatusBadRequest)
			return
		}

		// Show the scope that will be granted (AuthorizeHandler adds offline_access).
		var clientScopes []string
		if client, cErr := store.GetClient(r.Context(), clientID); cErr == nil {
			clientScopes = client.GetScopes()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"client_name": name,
			"scope":       effectiveScope(r.URL.Query().Get("scope"), clientScopes),
			"client_id":   clientID,
		})
	}
}
