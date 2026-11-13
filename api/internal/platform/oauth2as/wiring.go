package oauth2as

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"

	fosite "authelia.com/provider/oauth2"
	"github.com/google/uuid"

	"tools.xdoubleu.com/internal/models"
	"tools.xdoubleu.com/internal/platform/account"
	"tools.xdoubleu.com/internal/platform/agentaccounts"
)

// Wiring bundles the embedded OAuth 2.1 / OIDC authorization server.
type Wiring struct {
	Store    *Store
	Provider fosite.Provider
	// OIDCKey signs ID tokens; its public half is served at JWKSPath.
	OIDCKey *rsa.PrivateKey
}

// OIDCKeyID is the key ID of OIDCKey.
func (w *Wiring) OIDCKeyID() string {
	return OIDCKeyID(w.OIDCKey)
}

// Routes the authorization server is served on.
const (
	AuthorizePath = "/oauth2/authorize"
	//nolint:gosec // route path, not a credential
	TokenPath       = "/oauth2/token"
	RegisterPath    = "/oauth2/register"
	ConsentInfoPath = "/oauth2/consent-info"

	JWKSPath                = "/oauth2/jwks"
	MetadataPath            = "/.well-known/oauth-authorization-server"
	OpenIDConfigurationPath = "/.well-known/openid-configuration"
)

// SessionUsers resolves the browser session behind the accessToken cookie.
type SessionUsers interface {
	GetUser(ctx context.Context, accessToken string) (*models.User, error)
}

// UserLookup reads a user's DB record.
type UserLookup interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
}

// NewSessionUserResolver resolves the accessToken cookie, with DB role and
// display name overlaid, for AuthorizeHandler to re-verify the session and
// build ID-token claims.
func NewSessionUserResolver(
	sessions SessionUsers,
	users UserLookup,
) SessionUserResolver {
	return func(r *http.Request) (ResolvedUser, bool) {
		cookie, err := r.Cookie("accessToken")
		if err != nil {
			return ResolvedUser{}, false //nolint:exhaustruct // not-found sentinel
		}
		user, err := sessions.GetUser(r.Context(), cookie.Value)
		if err != nil {
			return ResolvedUser{}, false //nolint:exhaustruct // not-found sentinel
		}

		resolved := ResolvedUser{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			IsAdmin:     user.Role == models.RoleAdmin,
		}

		// The bare auth-schema user always has RoleUser and no display name.
		if dbUser, dbErr := users.GetByID(r.Context(), user.ID); dbErr == nil {
			resolved.Email = dbUser.Email
			resolved.DisplayName = dbUser.DisplayName
			resolved.IsAdmin = dbUser.Role == models.RoleAdmin
		}

		return resolved, true
	}
}

// MetadataHandler serves the RFC 8414 metadata (also the OIDC discovery
// document) for issuer.
func MetadataHandler(issuer string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                   issuer,
			"authorization_endpoint":   issuer + AuthorizePath,
			"token_endpoint":           issuer + TokenPath,
			"registration_endpoint":    issuer + RegisterPath,
			"jwks_uri":                 issuer + JWKSPath,
			"response_types_supported": []string{"code"},
			"grant_types_supported": []string{
				"authorization_code",
				"refresh_token",
				"client_credentials",
			},
			// Dynamic (MCP) clients only get offline_access; the OIDC scopes are for the
			// static Grafana SSO client.
			"scopes_supported":                      SupportedScopes,
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"code_challenge_methods_supported":      []string{"S256"},
			"token_endpoint_auth_methods_supported": []string{
				"none", "client_secret_basic", "client_secret_post",
			},
		})
	}
}

// agentDelegator lets the consent page authorize a client as an agent account.
type agentDelegator struct {
	agents *agentaccounts.Store
	users  *account.AppUsersRepository
}

// NewAgentDelegator delegates consent to agent accounts held by agents.
func NewAgentDelegator(
	agents *agentaccounts.Store,
	users *account.AppUsersRepository,
) Delegator {
	return agentDelegator{agents: agents, users: users}
}

func (d agentDelegator) Delegate(
	ctx context.Context, admin ResolvedUser, clientID, accountID string,
) (ResolvedUser, error) {
	id, err := uuid.Parse(accountID)
	if err != nil {
		return ResolvedUser{}, err
	}
	account, err := d.agents.Delegate(ctx, id, clientID, "human:"+admin.ID)
	if err != nil {
		return ResolvedUser{}, err
	}
	user, err := d.users.GetByID(ctx, account.UserID.String())
	if err != nil {
		return ResolvedUser{}, err
	}
	return ResolvedUser{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		IsAdmin:     false,
	}, nil
}

func (d agentDelegator) Undelegate(ctx context.Context, clientID string) error {
	return d.agents.Undelegate(ctx, clientID)
}
