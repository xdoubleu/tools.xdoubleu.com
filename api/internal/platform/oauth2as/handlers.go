package oauth2as

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	fosite "authelia.com/provider/oauth2"
	"authelia.com/provider/oauth2/handler/openid"

	"tools.xdoubleu.com/internal/config"
)

// SessionUserResolver resolves the web session's user from its cookie. It is
// injected by cmd/api to avoid an auth <-> oauth2as import cycle.
type SessionUserResolver func(r *http.Request) (user ResolvedUser, ok bool)

// AgentAccountParam names the agent account an admin delegates the client to.
const AgentAccountParam = "agent_account_id"

// Delegator binds a client to an agent account, so its tokens act as the
// agent rather than the admin who approved (adr-0048).
type Delegator interface {
	// Delegate returns the agent user the client is now authorized as.
	Delegate(
		ctx context.Context, admin ResolvedUser, clientID, accountID string,
	) (ResolvedUser, error)
	// Undelegate ends the client's delegation when it is authorized as a person.
	Undelegate(ctx context.Context, clientID string) error
}

// AuthorizeHandler serves /oauth2/authorize. A GET, or a request without a
// decision, redirects to the web consent page. A POST with consent=deny
// returns access_denied; consent=allow completes only with the consent token
// that page obtained for this user and request. An admin's approval with
// AgentAccountParam authorizes the client as that agent account instead.
func AuthorizeHandler(
	provider fosite.Provider,
	cfg config.Config,
	resolveUser SessionUserResolver,
	delegator Delegator,
	signingKeyID string,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		consent := r.URL.Query().Get("consent")

		if consent == "" || r.Method != http.MethodPost {
			query := r.URL.Query()
			query.Del("consent")
			//nolint:gosec // base URL is the server's own cfg.WebURL, not
			// user input — only the query string (the OAuth request
			// parameters the consent page needs) is taken from the request.
			http.Redirect(
				w, r, cfg.WebURL+"/oauth/consent?"+query.Encode(), http.StatusFound,
			)
			return
		}

		// The consent page decides through fetch, which can't read a redirect's
		// Location, so it asks for the destination as JSON instead.
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w = &locationAsJSON{ResponseWriter: w, captured: false}
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
		if !ok || !verifyConsentToken(
			[]byte(cfg.OAuthHMACSecret), user.ID, r.URL.Query(),
			r.Header.Get(ConsentTokenHeader), time.Now(),
		) {
			logOAuthError(
				ctx, logger, endpointAuthorize, ar, fosite.ErrRequestUnauthorized,
			)
			provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrRequestUnauthorized)
			return
		}

		subject, err := authorizeAs(
			ctx, delegator, user, ar.GetClient().GetID(),
			r.URL.Query().Get(AgentAccountParam),
		)
		if err != nil {
			logOAuthError(ctx, logger, endpointAuthorize, ar, err)
			provider.WriteAuthorizeError(ctx, w, ar, err)
			return
		}

		// Grant the requested scopes: fosite only issues a refresh token when
		// offline_access is granted, not merely requested.
		for _, scope := range ar.GetRequestedScopes() {
			ar.GrantScope(scope)
		}
		grantOfflineAccess(ar)

		session := newAuthorizeSession(subject, signingKeyID, ar.GetGrantedScopes())
		resp, err := provider.NewAuthorizeResponse(ctx, ar, session)
		if err != nil {
			logOAuthError(ctx, logger, endpointAuthorize, ar, err)
			provider.WriteAuthorizeError(ctx, w, ar, err)
			return
		}

		provider.WriteAuthorizeResponse(ctx, w, ar, resp)
	}
}

// authorizeAs is who the approved client acts as: the agent account an admin
// delegated it to, or the approving user.
func authorizeAs(
	ctx context.Context,
	delegator Delegator,
	user ResolvedUser,
	clientID, accountID string,
) (ResolvedUser, error) {
	if accountID == "" {
		if delegator == nil {
			return user, nil
		}
		if err := delegator.Undelegate(ctx, clientID); err != nil {
			return ResolvedUser{}, fosite.ErrServerError.WithWrap(err)
		}
		return user, nil
	}
	if delegator == nil || !user.IsAdmin {
		return ResolvedUser{}, fosite.ErrAccessDenied.WithHint(
			"Only an admin can delegate to an agent account.",
		)
	}
	agent, err := delegator.Delegate(ctx, user, clientID, accountID)
	if err != nil {
		return ResolvedUser{}, fosite.ErrInvalidRequest.
			WithHint("The agent account can't be delegated to.").WithWrap(err)
	}
	return agent, nil
}

// locationAsJSON replies to the redirect fosite writes with a 200 and
// {"location": "..."}; any other response passes through unchanged.
type locationAsJSON struct {
	http.ResponseWriter
	captured bool
}

func (l *locationAsJSON) WriteHeader(code int) {
	location := l.Header().Get("Location")
	redirect := code >= http.StatusMultipleChoices && code < http.StatusBadRequest
	if !redirect || location == "" {
		l.ResponseWriter.WriteHeader(code)
		return
	}

	l.captured = true
	l.Header().Del("Location")
	l.Header().Set("Content-Type", "application/json")
	l.ResponseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(l.ResponseWriter).Encode(map[string]string{"location": location})
}

func (l *locationAsJSON) Write(p []byte) (int, error) {
	if l.captured {
		return len(p), nil
	}
	return l.ResponseWriter.Write(p)
}

// TokenHandler serves /oauth2/token (authorization_code, refresh_token, and
// client_credentials for machine clients).
func TokenHandler(
	provider fosite.Provider,
	subjects MachineSubjects,
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
			subject, ok, lookupErr := subjects(ctx, ar.GetClient().GetID())
			if lookupErr != nil {
				err = fosite.ErrServerError.WithWrap(lookupErr)
				logOAuthError(ctx, logger, endpointToken, ar, err)
				provider.WriteAccessError(ctx, w, ar, err)
				return
			}
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
// client name and the scope approval will grant, plus, for a signed-in caller,
// the consent token that approving requires and whether they may delegate.
func ConsentInfoHandler(
	store *Store,
	cfg config.Config,
	resolveUser SessionUserResolver,
) http.HandlerFunc {
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

		info := map[string]any{
			"client_name": name,
			"scope":       effectiveScope(r.URL.Query().Get("scope"), clientScopes),
			"client_id":   clientID,
		}
		if user, ok := resolveUser(r); ok {
			info["consent_token"] = newConsentToken(
				[]byte(cfg.OAuthHMACSecret), user.ID, r.URL.Query(), time.Now(),
			)
			info["can_delegate"] = user.IsAdmin
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	}
}
