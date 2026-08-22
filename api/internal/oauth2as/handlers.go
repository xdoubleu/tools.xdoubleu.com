package oauth2as

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ory/fosite"

	"tools.xdoubleu.com/internal/config"
)

// SessionUserResolver resolves the currently-authenticated web session's
// user ID from the request's own session cookie — passed in from cmd/api
// (the composition root, which is the only place both internal/auth and
// internal/oauth2as are wired together) rather than imported directly here,
// to avoid a cycle between the two packages.
type SessionUserResolver func(r *http.Request) (userID string, ok bool)

// AuthorizeHandler implements the /oauth2/authorize endpoint. The first hit
// (no consent decision yet) redirects to the web app's own consent page;
// consent=allow completes the flow (re-verifying the session server-side as
// defense in depth), consent=deny writes a standard access_denied error.
func AuthorizeHandler(
	provider fosite.OAuth2Provider,
	cfg config.Config,
	resolveUser SessionUserResolver,
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
			provider.WriteAuthorizeError(ctx, w, ar, err)
			return
		}

		if consent == "deny" {
			provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrAccessDenied)
			return
		}

		userID, ok := resolveUser(r)
		if !ok {
			provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrRequestUnauthorized)
			return
		}

		// Consent has just been granted for exactly the requested scopes —
		// without this, GrantedScope stays empty and the token endpoint
		// never issues a refresh token (canIssueRefreshToken requires
		// offline_access among the *granted*, not merely requested, scopes),
		// silently breaking every client despite offline_access being on
		// the registered client's scope list.
		for _, scope := range ar.GetRequestedScopes() {
			ar.GrantScope(scope)
		}
		grantOfflineAccess(ar)

		//nolint:exhaustruct //other DefaultSession fields are optional
		session := &fosite.DefaultSession{Subject: userID}
		resp, err := provider.NewAuthorizeResponse(ctx, ar, session)
		if err != nil {
			provider.WriteAuthorizeError(ctx, w, ar, err)
			return
		}

		provider.WriteAuthorizeResponse(ctx, w, ar, resp)
	}
}

// TokenHandler implements the /oauth2/token endpoint (authorization_code and
// refresh_token grants).
func TokenHandler(provider fosite.OAuth2Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		//nolint:exhaustruct //other DefaultSession fields are optional
		session := &fosite.DefaultSession{}
		ar, err := provider.NewAccessRequest(ctx, r, session)
		if err != nil {
			provider.WriteAccessError(ctx, w, ar, err)
			return
		}

		resp, err := provider.NewAccessResponse(ctx, ar)
		if err != nil {
			provider.WriteAccessError(ctx, w, ar, err)
			return
		}

		provider.WriteAccessResponse(ctx, w, ar, resp)
	}
}

// registerRequest/registerResponse are the RFC 7591 request/response shapes
// this server accepts and returns.
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
	// Scope is the space-delimited scope the server assigned this client
	// (RFC 7591 §3.2.1). Echoing it is what tells a client to ask for
	// offline_access, and so to be issued a refresh token.
	Scope string `json:"scope"`
}

// RegisterHandler implements RFC 7591 dynamic client registration at
// /oauth2/register.
func RegisterHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		client, err := RegisterClient(r.Context(), store.db, ClientMetadata(req))
		if err != nil {
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

// ConsentInfoHandler implements GET /oauth2/consent-info: echoes back the
// pending authorization request's client name and the scope that approving it
// will actually grant, for the web consent page to display before it POSTs
// the user's decision back to /oauth2/authorize.
func ConsentInfoHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		name, err := store.GetClientName(r.Context(), clientID)
		if err != nil {
			http.Error(w, "unknown client", http.StatusBadRequest)
			return
		}

		// The scope shown is the one that will actually be granted, not the
		// raw request parameter — AuthorizeHandler adds offline_access on top
		// of whatever the client asked for.
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
