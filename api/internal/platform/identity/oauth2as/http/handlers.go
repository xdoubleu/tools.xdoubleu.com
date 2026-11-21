// Package http serves the authorization server's HTTP endpoints (authorize,
// token, register, consent info, metadata). It maps requests and errors to
// fosite; the consent rules live in oauth2as.Authorizer.
package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"strings"

	fosite "authelia.com/provider/oauth2"
	"authelia.com/provider/oauth2/handler/openid"

	"tools.xdoubleu.com/internal/platform/identity/oauth2as"
)

// SessionUserResolver resolves the web session's user from its cookie.
type SessionUserResolver func(r *nethttp.Request) (user oauth2as.ResolvedUser, ok bool)

// AgentAccountParam names the agent account an admin delegates the client to.
const AgentAccountParam = "agent_account_id"

// AuthorizeHandler serves /oauth2/authorize. A GET, or a request without a
// decision, redirects to the web consent page, unless the signed-in user's
// remembered grant for the client covers the requested scopes. A POST with
// consent=deny returns access_denied; consent=allow completes only with the
// consent token that page obtained for this user and request. An admin's
// approval with AgentAccountParam authorizes the client as that agent account
// instead.
func AuthorizeHandler(
	provider fosite.Provider,
	webURL string,
	resolveUser SessionUserResolver,
	authz *oauth2as.Authorizer,
	signingKeyID string,
	logger *slog.Logger,
) nethttp.HandlerFunc {
	d := authorizeDeps{
		provider: provider, resolveUser: resolveUser, authz: authz,
		signingKeyID: signingKeyID, logger: logger,
	}
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		ctx := r.Context()
		consent := r.URL.Query().Get("consent")

		remembered := (consent == "" || r.Method != nethttp.MethodPost) &&
			d.rememberedGrant(r)

		if !remembered && (consent == "" || r.Method != nethttp.MethodPost) {
			redirectToConsent(w, r, webURL)
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

		if !remembered && consent == "deny" {
			// Declining is normal; logged only for visibility.
			logOAuthError(
				ctx, logger, endpointAuthorize, ar, fosite.ErrAccessDenied,
			)
			provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrAccessDenied)
			return
		}

		d.complete(w, r, ar, remembered)
	}
}

// authorizeDeps are AuthorizeHandler's collaborators.
type authorizeDeps struct {
	provider     fosite.Provider
	resolveUser  SessionUserResolver
	authz        *oauth2as.Authorizer
	signingKeyID string
	logger       *slog.Logger
}

// complete issues the code for an approved request: remembered, or carrying a
// consent token for this user and request.
func (d authorizeDeps) complete(
	w nethttp.ResponseWriter,
	r *nethttp.Request,
	ar fosite.AuthorizeRequester,
	remembered bool,
) {
	ctx := r.Context()
	provider, logger := d.provider, d.logger
	user, ok := d.resolveUser(r)
	if !ok || (!remembered && !d.authz.VerifyConsent(
		user.ID, r.URL.Query(), r.Header.Get(oauth2as.ConsentTokenHeader),
	)) {
		logOAuthError(
			ctx, logger, endpointAuthorize, ar, fosite.ErrRequestUnauthorized,
		)
		provider.WriteAuthorizeError(ctx, w, ar, fosite.ErrRequestUnauthorized)
		return
	}

	clientID := ar.GetClient().GetID()
	accountID := r.URL.Query().Get(AgentAccountParam)
	subject, err := d.authz.Subject(ctx, user, clientID, accountID)
	if err != nil {
		err = delegationError(err)
		logOAuthError(ctx, logger, endpointAuthorize, ar, err)
		provider.WriteAuthorizeError(ctx, w, ar, err)
		return
	}

	grantRequestedScopes(ar)

	if !remembered {
		d.authz.Remember(ctx, user.ID, clientID, ar.GetGrantedScopes(), accountID)
	}

	session := oauth2as.NewAuthorizeSession(
		subject, d.signingKeyID, ar.GetGrantedScopes(),
	)
	resp, err := provider.NewAuthorizeResponse(ctx, ar, session)
	if err != nil {
		logOAuthError(ctx, logger, endpointAuthorize, ar, err)
		provider.WriteAuthorizeError(ctx, w, ar, err)
		return
	}

	provider.WriteAuthorizeResponse(ctx, w, ar, resp)
}

// rememberedGrant reports whether a signed-in user already approved this
// client for every scope the request would grant, so a plain page load skips
// the consent page. Any failure means "no".
func (d authorizeDeps) rememberedGrant(r *nethttp.Request) bool {
	if r.Method != nethttp.MethodGet {
		return false
	}
	user, ok := d.resolveUser(r)
	if !ok {
		return false
	}
	ar, err := d.provider.NewAuthorizeRequest(r.Context(), r)
	if err != nil {
		return false
	}
	grantRequestedScopes(ar)
	return d.authz.Remembered(
		r.Context(), user.ID, ar.GetClient().GetID(), ar.GetGrantedScopes(),
		r.URL.Query().Get(AgentAccountParam),
	)
}

func redirectToConsent(w nethttp.ResponseWriter, r *nethttp.Request, webURL string) {
	query := r.URL.Query()
	query.Del("consent")
	//nolint:gosec // base URL is the server's own web URL, not user input —
	// only the query string (the OAuth request parameters the consent page
	// needs) is taken from the request.
	nethttp.Redirect(w, r, webURL+"/oauth/consent?"+query.Encode(), nethttp.StatusFound)
}

// grantRequestedScopes grants every requested scope: fosite only issues a
// refresh token when offline_access is granted, not merely requested.
func grantRequestedScopes(ar fosite.AuthorizeRequester) {
	for _, scope := range ar.GetRequestedScopes() {
		ar.GrantScope(scope)
	}
	oauth2as.GrantOfflineAccess(ar)
}

// delegationError maps an Authorizer.Subject failure to an OAuth error.
func delegationError(err error) error {
	switch {
	case errors.Is(err, oauth2as.ErrDelegationForbidden):
		return fosite.ErrAccessDenied.WithHint(
			"Only an admin can delegate to an agent account.",
		)
	case errors.Is(err, oauth2as.ErrDelegationFailed):
		return fosite.ErrInvalidRequest.
			WithHint("The agent account can't be delegated to.").WithWrap(err)
	}
	return fosite.ErrServerError.WithWrap(err)
}

// locationAsJSON replies to the redirect fosite writes with a 200 and
// {"location": "..."}; any other response passes through unchanged.
type locationAsJSON struct {
	nethttp.ResponseWriter
	captured bool
}

func (l *locationAsJSON) WriteHeader(code int) {
	location := l.Header().Get("Location")
	redirect := code >= nethttp.StatusMultipleChoices && code < nethttp.StatusBadRequest
	if !redirect || location == "" {
		l.ResponseWriter.WriteHeader(code)
		return
	}

	l.captured = true
	l.Header().Del("Location")
	l.Header().Set("Content-Type", "application/json")
	l.ResponseWriter.WriteHeader(nethttp.StatusOK)
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
	subjects oauth2as.MachineSubjects,
	logger *slog.Logger,
) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
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
func RegisterHandler(store *oauth2as.Store, logger *slog.Logger) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		ctx := r.Context()

		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logRegisterError(
				ctx, logger, errors.New("malformed registration body"),
			)
			nethttp.Error(w, "invalid request body", nethttp.StatusBadRequest)
			return
		}

		client, err := store.Register(ctx, oauth2as.ClientMetadata(req))
		if err != nil {
			logRegisterError(ctx, logger, err)
			nethttp.Error(w, err.Error(), nethttp.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(nethttp.StatusCreated)
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
	store *oauth2as.Store,
	authz *oauth2as.Authorizer,
	resolveUser SessionUserResolver,
) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		clientID := r.URL.Query().Get("client_id")
		name, err := store.GetClientName(r.Context(), clientID)
		if err != nil {
			nethttp.Error(w, "unknown client", nethttp.StatusBadRequest)
			return
		}

		// Show the scope that will be granted (AuthorizeHandler adds offline_access).
		var clientScopes []string
		if client, cErr := store.GetClient(r.Context(), clientID); cErr == nil {
			clientScopes = client.GetScopes()
		}

		info := map[string]any{
			"client_name": name,
			"scope":       oauth2as.EffectiveScope(r.URL.Query().Get("scope"), clientScopes),
			"client_id":   clientID,
		}
		if user, ok := resolveUser(r); ok {
			info["consent_token"] = authz.NewConsentToken(user.ID, r.URL.Query())
			info["can_delegate"] = user.IsAdmin
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	}
}
